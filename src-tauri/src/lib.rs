use std::sync::Mutex;
use tauri::Manager;
use tauri_plugin_shell::{process::CommandChild, process::CommandEvent, ShellExt};

struct SidecarState {
    child: Mutex<Option<CommandChild>>,
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    // 修复 NVIDIA + WebKitGTK 兼容性问题
    std::env::set_var("GDK_BACKEND", "x11");
    std::env::set_var("WEBKIT_DISABLE_DMABUF_RENDERER", "1");

    let app = tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .setup(|app| {
            // 启动 Go 后端 sidecar
            let sidecar = app.shell().sidecar("go-backend").unwrap();
            let (rx, child) = sidecar.spawn().unwrap();

            // 监听 sidecar 输出并转发到 Tauri 日志系统
            tauri::async_runtime::spawn(async move {
                let mut rx = rx;
                while let Some(event) = rx.recv().await {
                    match event {
                        CommandEvent::Stdout(line) => {
                            log::info!("[go-backend] {}", String::from_utf8_lossy(&line));
                        }
                        CommandEvent::Stderr(line) => {
                            log::warn!("[go-backend] {}", String::from_utf8_lossy(&line));
                        }
                        CommandEvent::Terminated(status) => {
                            log::info!(
                                "[go-backend] process exited with status: {:?}",
                                status.code
                            );
                            break;
                        }
                        _ => {}
                    }
                }
            });

            // 保存 child 进程句柄以便优雅退出
            app.manage(SidecarState {
                child: Mutex::new(Some(child)),
            });

            if cfg!(debug_assertions) {
                app.handle().plugin(
                    tauri_plugin_log::Builder::default()
                        .level(log::LevelFilter::Info)
                        .build(),
                )?;
            }
            Ok(())
        })
        .build(tauri::generate_context!())
        .expect("error while building tauri application");

    // 运行应用，在退出时杀掉 Go 后端子进程
    app.run(|app_handle, event| {
        if let tauri::RunEvent::ExitRequested { .. } = event {
            if let Some(state) = app_handle.try_state::<SidecarState>() {
                if let Some(child) = state.inner().child.lock().unwrap().take() {
                    if let Err(e) = child.kill() {
                        log::error!("failed to kill go-backend sidecar: {}", e);
                    } else {
                        log::info!("go-backend sidecar killed successfully");
                    }
                }
            }
        }
    });
}
