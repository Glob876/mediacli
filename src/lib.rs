pub mod cli;
pub mod config;
pub mod daemon;
pub mod download;
pub mod media;
pub mod queue;
pub mod remote;
mod settings;
pub mod storage;
pub mod theme;
pub mod tools;
pub mod tui;

pub const VERSION: &str = env!("CARGO_PKG_VERSION");
