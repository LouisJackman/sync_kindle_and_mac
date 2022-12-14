// The use of Tokio is probably slower than using blocking calls directly, due to the lack of truly
// asynchronous filesystem I/O APIs on some OSes. That said, using it means a threadpool doesn't
// need to be imported or written, and perhaps Tokio will, one day, transparently support the likes
// of `io_uring` for their filesystem APIs.

#![forbid(unsafe_code)]

use {
    anyhow::{anyhow, Error, Result},
    async_walkdir::WalkDir,
    clap::Parser,
    directories::UserDirs,
    std::{
        collections::HashSet,
        ffi::OsStr,
        path::{Path, PathBuf},
        sync::Arc,
    },
    tokio::{
        self,
        fs::{self, File},
        io::{self, stdout, AsyncWriteExt},
        sync::mpsc::{channel, Receiver, Sender},
        task::{spawn, JoinHandle},
    },
    tokio_stream::StreamExt,
    whoami::username,
};

const NAME: &str = "sync-kobo-and-workstation";

const LONG_ABOUT: &str = "Synchronise books between a workstation and a Kobo e-book reader. In \
                          practice, this means synchronising a connected Kobo volume with EPUB \
                          and PDF files in the specified local documents directories. The \
                          defaults are Linux-specific, assuming a udisks2-style automount \
                          directory such as /var/media/user/KOBOeReader for the destination Kobo \
                          and defaulting to just ~/Documents for the source. However, if these \
                          defaults are overridden with explicit values, it will likely work on \
                          other OSes too.";

const EXTENSIONS_TO_SYNCHRONISE: [&str; 2] = ["epub", ".pdf"];

const FOUND_BOOKS_CHANNEL_BOUND: usize = 128;
const STATISTICS_CHANNEL_BOUND: usize = 128;

macro_rules! println_async {
    ($fmt:literal $(, $elem:expr )* $(,)?) => {
        {
            let msg = format!($fmt, $( $elem, )*);
            stdout().write_all(msg.as_bytes()).await?;
            stdout().write_all(b"\n")
        }
    };
}

#[derive(Debug)]
enum Statistic {
    FoundSrcDocument,
    NotCopiedBecauseAlreadyExistedAtDest,
    Copied,
}

async fn is_accessible_dir(path: &Path) -> bool {
    fs::metadata(path)
        .await
        .map(|m| m.is_dir())
        .unwrap_or(false)
}

fn lookup_default_kobo_storage_directory() -> PathBuf {
    let mut buf = PathBuf::new();
    buf.push("/media");
    buf.push(username());
    buf.push("KOBOeReader");
    buf
}

fn lookup_home_directory() -> Result<PathBuf> {
    let dirs =
        UserDirs::new().ok_or_else(|| anyhow!("failed to read the current home directory"))?;

    let home = dirs.home_dir();
    Ok(home.to_path_buf())
}

fn lookup_default_documents_directories() -> Result<Vec<PathBuf>> {
    let home = lookup_home_directory()?;

    let mut documents = PathBuf::new();
    documents.push(home);
    documents.push("Documents");

    Ok(vec![documents])
}

async fn find_books(
    dirs: &[PathBuf],
    extensions_to_match: &HashSet<&OsStr>,
    books: Sender<PathBuf>,
    stats: Sender<Statistic>,
) -> Result<()> {
    for dir in dirs {
        let mut entries = WalkDir::new(dir);
        loop {
            match entries.next().await {
                Some(Ok(entry)) => {
                    let path = entry.path();
                    if let Some(ext) = path.extension() {
                        if extensions_to_match.contains(&ext) {
                            stats.send(Statistic::FoundSrcDocument).await?;

                            let path_buf = path.to_path_buf();
                            books.send(path_buf).await?;
                        }
                    }
                }
                Some(Err(err)) => Err(anyhow!(err))?,
                None => break,
            }
        }
    }
    Ok(())
}

fn path_str(path: &Path) -> Result<&str> {
    path.to_str()
        .ok_or_else(|| anyhow!("could not decode a path to UTF-8"))
}

async fn copy_to_non_existant(
    src_path: &Path,
    dest_path: &Path,
    dry_run: bool,
) -> Result<JoinHandle<Result<()>>> {
    if dry_run {
        let (src, dest) = (path_str(src_path)?, path_str(dest_path)?);
        println_async!("Dry-running; would otherwise copy {src} to {dest}").await?;
        Ok(spawn(async { Ok(()) }))
    } else {
        let mut src = File::open(src_path).await?;

        let mut dest = fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .open(dest_path)
            .await?;

        let src_str = path_str(src_path)?.to_owned();
        let dest_str = path_str(dest_path)?.to_owned();

        Ok(tokio::spawn(async move {
            io::copy(&mut src, &mut dest).await?;
            println_async!("Copied {src_str} to {dest_str}").await?;
            Ok(())
        }))
    }
}

async fn sync_books(
    dest_dir: &Path,
    dry_run: bool,
    mut books_to_sync: Receiver<PathBuf>,
    stats: Sender<Statistic>,
) -> Result<()> {
    let mut copy_tasks = vec![];

    while let Some(book) = books_to_sync.recv().await {
        let mut dest_path = PathBuf::new();
        dest_path.push(dest_dir);

        if let Some(book_name) = book.file_name() {
            dest_path.push(book_name);

            if let Ok(copy_op) = copy_to_non_existant(&book, &dest_path, dry_run).await {
                copy_tasks.push(copy_op);
                stats.send(Statistic::Copied).await?;
            } else {
                let dest_str = path_str(&dest_path)?;
                println_async!(
                    "Book {dest_str} already exists on the destination; will not copy across."
                )
                .await?;
                stats
                    .send(Statistic::NotCopiedBecauseAlreadyExistedAtDest)
                    .await?;
            }
        }
    }

    for task in copy_tasks {
        task.await??;
    }

    Ok(())
}

#[derive(Debug, Parser)]
#[command(name = NAME, about, author, version, long_about = LONG_ABOUT)]
struct PartialArgs {
    /// The directory of the mounted Kobo storage directory to which to synchronise the books and
    /// documents.
    #[arg(long)]
    kobo_directory: Option<PathBuf>,

    /// The directory of the documents directories from which to synchronise books and documents.
    #[arg(long)]
    documents_directories: Option<Vec<PathBuf>>,

    /// Whether to dry run, documenting what would happen rather than doing it.
    #[arg(long, default_value_t = false)]
    dry_run: bool,
}

struct Args {
    kobo_directory: PathBuf,
    documents_directories: Vec<PathBuf>,
    dry_run: bool,
}

async fn parse_args() -> Result<Args> {
    let partial @ PartialArgs { dry_run, .. } = PartialArgs::parse();

    let kobo_directory = partial
        .kobo_directory
        .unwrap_or_else(lookup_default_kobo_storage_directory);

    let documents_directories = partial.documents_directories.unwrap_or_else(|| {
        lookup_default_documents_directories().expect(
            "failed to lookup the default documents directory while yielding a default \
                    value for that missing argument",
        )
    });

    if !is_accessible_dir(&kobo_directory).await {
        let inaccessible = kobo_directory.to_str().ok_or_else(|| {
            anyhow!("could not decode Kobo directory path as UTF-8 while reporting its absense")
        })?;
        return Err(anyhow!(
            "The Kobo storage directory at {inaccessible} is not accessible"
        ));
    }
    for dir in &documents_directories {
        if !is_accessible_dir(dir).await {
            let inaccessible = dir.to_str().ok_or_else(|| {
                anyhow!(
                    "could not a decode documents directory path as UTF-8 while reporting its \
                        absence",
                )
            })?;
            return Err(anyhow!(
                "The documents directory at {inaccessible} is not accessible"
            ));
        }
    }

    Ok(Args {
        kobo_directory,
        documents_directories,
        dry_run,
    })
}

async fn collect_stats(dest_dirs: &[PathBuf], mut stats: Receiver<Statistic>) -> Result<()> {
    let mut found_src_documents: usize = 0;
    let mut not_copied: usize = 0;
    let mut copied: usize = 0;

    while let Some(stat) = stats.recv().await {
        use Statistic::*;
        match stat {
            FoundSrcDocument => {
                found_src_documents += 1;
            }
            NotCopiedBecauseAlreadyExistedAtDest => {
                not_copied += 1;
            }
            Copied => {
                copied += 1;
            }
        }
    }

    let len = dest_dirs.len();
    let dest_str: String =
        dest_dirs
            .iter()
            .zip(1..)
            .try_fold(String::new(), |mut s, (dir, i)| {
                s.push_str(path_str(dir)?);
                if i < len {
                    s.push_str(" and ");
                }
                Ok::<String, Error>(s)
            })?;

    println_async!(
        "\n\
        Found documents in documents directory at {dest_str}: {found_src_documents}\n\
        Books not copied because they already exist on the destination Kobo: {not_copied}\n\
        Book copied: {copied}"
    )
    .await?;

    Ok(())
}

#[tokio::main]
async fn main() -> Result<(), Error> {
    let Args {
        dry_run,
        kobo_directory,
        documents_directories,
    } = parse_args().await?;

    let extensions: HashSet<&OsStr> = EXTENSIONS_TO_SYNCHRONISE.iter().map(OsStr::new).collect();

    let (book_path_tx, book_path_rx) = channel::<PathBuf>(FOUND_BOOKS_CHANNEL_BOUND);
    let (stats_tx, stats_rx) = channel::<Statistic>(STATISTICS_CHANNEL_BOUND);

    let documents_directories_ptr = Arc::new(documents_directories);

    let stats_collection = {
        let documents_directories_ptr = documents_directories_ptr.clone();
        spawn(async move { collect_stats(&(*documents_directories_ptr)[..], stats_rx).await })
    };

    let book_finding = {
        let stats_tx = stats_tx.clone();
        spawn(async move {
            find_books(
                &(*documents_directories_ptr)[..],
                &extensions,
                book_path_tx,
                stats_tx,
            )
            .await
        })
    };

    sync_books(&kobo_directory, dry_run, book_path_rx, stats_tx).await?;
    book_finding.await??;
    stats_collection.await??;

    Ok(())
}
