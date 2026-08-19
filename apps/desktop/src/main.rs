#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]
// Tauri 桌面壳：spawn Node sidecar（server.cjs）+ 开窗指向 127.0.0.1:PORT + 托盘。
// 退出无痕：退出时杀 sidecar 子进程，sidecar 收 SIGTERM 后优雅停止 nginx。

use std::net::TcpStream;
use std::path::PathBuf;
use std::process::{Command, Stdio};
use std::sync::Mutex;
use std::time::{Duration, Instant};

use tauri::{
    menu::{Menu, MenuItem},
    tray::TrayIconBuilder,
    AppHandle, Emitter, Manager, WebviewWindowBuilder,
};
use tauri_plugin_dialog::{DialogExt, MessageDialogKind};

const PORT: u16 = 47831;

/// 检查端口是否就绪
fn port_up(p: u16) -> bool {
    TcpStream::connect(("127.0.0.1", p)).is_ok()
}

/// 轮询等待端口就绪（最多 ms 毫秒）
fn wait_port(p: u16, ms: u64) -> bool {
    let dl = Instant::now() + Duration::from_millis(ms);
    while Instant::now() < dl {
        if port_up(p) {
            return true;
        }
        std::thread::sleep(Duration::from_millis(300));
    }
    false
}

/// 获取 data 目录根（exe 旁的 data/ 目录）
fn data_root(_app: &AppHandle) -> PathBuf {
    // Windows: exe 旁目录；macOS: .app 父目录
    let exe = std::env::current_exe().unwrap_or_else(|_| PathBuf::from("."));
    let dir = if cfg!(target_os = "macos") {
        // Foo.app/Contents/MacOS/Foo → Foo.app 父目录
        exe.parent()
            .and_then(|p| p.parent())
            .and_then(|p| p.parent())
            .map(|p| p.to_path_buf())
            .unwrap_or_else(|| exe.parent().unwrap().to_path_buf())
    } else {
        exe.parent().unwrap().to_path_buf()
    };
    dir
}

/// 获取 resource 目录（Tauri 注入的 resource_dir）
fn resource_dir(app: &AppHandle) -> PathBuf {
    app.path()
        .resource_dir()
        .unwrap_or_else(|_| PathBuf::from("."))
}

/// Windows 中文路径检查：nginx.exe 用 ANSI 代码页读文件，路径含非 ASCII 字符会失败
fn check_chinese_path(app: &AppHandle) -> bool {
    if !cfg!(target_os = "windows") {
        return true;
    }
    let root = data_root(app);
    let path_str = root.to_string_lossy();
    if path_str.chars().any(|c| !c.is_ascii()) {
        let _ = app
            .dialog()
            .message(format!(
                "本程序内置的 nginx 不支持包含中文的路径。\n\n当前路径：\n{}\n\n解决方法：把整个程序文件夹复制或移动到纯英文路径下（例如 D:\\nginx-jump\\），再重新运行。",
                path_str
            ))
            .title("路径包含中文，无法启动")
            .kind(MessageDialogKind::Error)
            .show(|_| {});
        return false;
    }
    true
}

/// 生成随机 secret（用于密码加密）
fn random_secret() -> String {
    use std::time::SystemTime;
    let now = SystemTime::now()
        .duration_since(SystemTime::UNIX_EPOCH)
        .unwrap()
        .as_nanos();
    format!("ngj-{}", now)
}

/// spawn Node sidecar 子进程
fn spawn_sidecar(app: &AppHandle) -> Option<std::process::Child> {
    let res = resource_dir(app);
    let data = data_root(app);

    // node 二进制路径
    let node_bin = if cfg!(target_os = "windows") {
        res.join("bin").join("node.exe")
    } else {
        res.join("bin").join("node")
    };

    // server.cjs 路径
    let server_cjs = res.join("server").join("server.cjs");

    // web-dist 路径（sidecar 托管静态文件）
    let web_dist = res.join("web-dist");

    if !node_bin.exists() {
        eprintln!("[startup] node binary not found: {:?}", node_bin);
        return None;
    }
    if !server_cjs.exists() {
        eprintln!("[startup] server.cjs not found: {:?}", server_cjs);
        return None;
    }

    let secret = random_secret();

    let child = Command::new(&node_bin)
        .arg(&server_cjs)
        .env("NGJ_DATA_ROOT", &data)
        .env("NGJ_RESOURCES_DIR", &res)
        .env("NGJ_PORT", PORT.to_string())
        .env("NGJ_WEB_DIST", &web_dist)
        .env("NGJ_SECRET", &secret)
        .stdout(Stdio::inherit())
        .stderr(Stdio::inherit())
        .spawn()
        .ok()?;

    Some(child)
}

#[tauri::command]
fn get_port() -> u16 {
    PORT
}

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| {
            // 单实例：已有实例运行时，聚焦窗口
            if let Some(win) = app.get_webview_window("main") {
                let _ = win.show();
                let _ = win.set_focus();
            }
        }))
        .setup(|app| {
            // Windows 中文路径检查
            if !check_chinese_path(app.handle()) {
                app.handle().exit(1);
                return Ok(());
            }

            // spawn sidecar
            let mut child = match spawn_sidecar(app.handle()) {
                Some(c) => c,
                None => {
                    eprintln!("[startup] failed to spawn sidecar");
                    app.handle().exit(1);
                    return Ok(());
                }
            };

            // 等待端口就绪（15s）
            if !wait_port(PORT, 15000) {
                eprintln!("[startup] sidecar port not ready after 15s");
                let _ = child.kill();
                app.handle().exit(1);
                return Ok(());
            }

            // 开窗指向 sidecar HTTP server
            let url = format!("http://127.0.0.1:{}/", PORT);
            let _win = WebviewWindowBuilder::new(app, "main", tauri::WebviewUrl::External(url.parse().unwrap()))
                .title("Nginx 跳板机")
                .inner_size(1100.0, 720.0)
                .min_inner_size(900.0, 600.0)
                .build()?;

            // 托盘
            let show_item = MenuItem::with_id(app, "show", "显示", true, None::<&str>)?;
            let quit_item = MenuItem::with_id(app, "quit", "退出", true, None::<&str>)?;
            let menu = Menu::with_items(app, &[&show_item, &quit_item])?;

            let _tray = TrayIconBuilder::new()
                .menu(&menu)
                .show_menu_on_left_click(false)
                .on_menu_event(|app, event| {
                    match event.id.as_ref() {
                        "show" => {
                            if let Some(win) = app.get_webview_window("main") {
                                let _ = win.show();
                                let _ = win.set_focus();
                            }
                        }
                        "quit" => {
                            app.emit("quit-requested", ()).ok();
                            app.exit(0);
                        }
                        _ => {}
                    }
                })
                .build(app)?;

            // 保存 sidecar child 到 app state，退出时 kill
            app.manage(SidecarState { child: Mutex::new(Some(child)) });

            Ok(())
        })
        .on_window_event(|window, event| {
            if let tauri::WindowEvent::CloseRequested { .. } = event {
                // 窗口关闭时退出 app（触发 RunEvent::Exit）
                window.app_handle().exit(0);
            }
        })
        .invoke_handler(tauri::generate_handler![get_port])
        .build(tauri::generate_context!())
        .expect("error building tauri app")
        .run(|app, event| {
            if let tauri::RunEvent::ExitRequested { .. } = event {
                // 退出时 kill sidecar
                if let Some(state) = app.try_state::<SidecarState>() {
                    if let Some(mut child) = state.child.lock().unwrap().take() {
                        let _ = child.kill();
                        let _ = child.wait();
                    }
                }
            }
        });
}

struct SidecarState {
    child: Mutex<Option<std::process::Child>>,
}

unsafe impl Send for SidecarState {}
unsafe impl Sync for SidecarState {}
