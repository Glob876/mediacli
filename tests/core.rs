use mediacli::{config, download, media, storage};
use serde_json::json;
#[test]
fn rejects_invalid_times_and_keeps_absolute_trim_end() {
    assert_eq!(media::parse_time("01:02:03.5").unwrap(), 3723.5);
    for text in ["NaN", "-1", "inf", "1:2:3:4", ""] {
        assert!(media::parse_time(text).is_err(), "{text}");
    }
}
#[test]
fn validates_typed_operation_fields() {
    assert!(media::validate_operation(&media::operation("scale", json!({"width":720}))).is_ok());
    assert!(media::validate_operation(&media::operation("scale", json!({"width":"720"}))).is_err());
    assert!(media::validate_operation(&media::operation("scale", json!({"unknown":720}))).is_err());
    assert!(media::validate_operation(&media::operation("unknown", json!({}))).is_err());
}
#[test]
fn download_preserves_quality_limits_and_secrets_are_masked() {
    let cfg = config::defaults();
    let plan=download::build("https://example.com/video",&json!({"quality":"1080","fps_limit":"30","vcodec":"av1","proxy_mode":"custom","proxy_url":"https://user:secret@proxy","video_password":"secret"}),&cfg,"",false,None).unwrap();
    let command = &plan.commands[0];
    let index = command.iter().position(|s| s == "-f").unwrap();
    for fallback in command[index + 1].split('/') {
        assert!(fallback.contains("[height<=1080]") && fallback.contains("[fps<=30]"));
    }
    assert!(!media::display_command(command).contains("secret"));
    assert!(command.contains(&"--no-simulate".to_string()));
}
#[test]
fn output_guards_protect_hardlinks_and_originals() {
    let dir = tempfile::tempdir().unwrap();
    let input = dir.path().join("input.mp4");
    let link = dir.path().join("link.mp4");
    std::fs::write(&input, b"original").unwrap();
    std::fs::hard_link(&input, &link).unwrap();
    assert!(
        media::protect_output(&media::text_path(&link), &[media::text_path(&input)], true).is_err()
    );
    let temporary = dir.path().join("temporary.mp4");
    std::fs::write(&temporary, b"partial").unwrap();
    let replacement = media::Replacement {
        input: media::text_path(&input),
        temporary: media::text_path(&temporary),
        final_path: media::text_path(&input),
    };
    replacement.finish(false).unwrap();
    assert_eq!(std::fs::read(&input).unwrap(), b"original");
    assert!(!temporary.exists());
    std::fs::write(&temporary, b"converted").unwrap();
    replacement.finish(true).unwrap();
    assert_eq!(std::fs::read(&input).unwrap(), b"converted");
}
#[test]
fn atomic_storage_does_not_lose_parallel_updates() {
    let directory = tempfile::tempdir().unwrap();
    let path = directory.path().join("history.json");
    std::thread::scope(|scope| {
        for index in 0..20 {
            let path = &path;
            scope.spawn(move || {
                storage::update_json(path, json!([]), |value| {
                    value.as_array_mut().unwrap().push(json!(index));
                    Ok(())
                })
                .unwrap();
            });
        }
    });
    let value = storage::read_json(&path, json!([])).unwrap();
    assert_eq!(value.as_array().unwrap().len(), 20);
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        assert_eq!(
            std::fs::metadata(path).unwrap().permissions().mode() & 0o777,
            0o600
        );
    }
}
