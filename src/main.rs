fn main() {
    let arguments: Vec<String> = std::env::args().skip(1).collect();
    let result = mediacli::cli::run(arguments);
    if let Err(error) = result {
        eprintln!("mediacli: {error:#}");
        let code = error
            .downcast_ref::<mediacli::cli::Exit>()
            .map(|status| status.0)
            .unwrap_or(1);
        std::process::exit(code);
    }
}
