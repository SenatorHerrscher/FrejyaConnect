use crossterm::{
    event::{self, DisableMouseCapture, EnableMouseCapture, Event, KeyCode},
    execute,
    terminal::{EnterAlternateScreen, LeaveAlternateScreen, disable_raw_mode, enable_raw_mode},
};

use ratatui::{
    Terminal,
    backend::CrosstermBackend,
    layout::{Constraint, Direction, Layout},
    style::{Color, Modifier, Style},
    widgets::{Block, Borders, Cell, Paragraph, Row, Table, TableState},
};
use serde::Deserialize;
use std::{error::Error, io, process::Command, string};

#[derive(Debug, Deserialize, Clone)]
//JSON ENCODER NEWCODER burada...............................
struct Device {
    #[serde(rename = "Ip")]
    ip: String,
    #[serde(rename = "Mac")]
    mac: String,
    #[serde(rename = "Vendor")]
    vendor: String,
    #[serde(rename = "OpenPorts")]
    open_ports: Option<Vec<u16>>,
    #[serde(rename = "DeviceType")]
    device_type: String,
}
// ...............................................................
struct App {
    devices: Vec<Device>,
    state: TableState,
    status_msg: String,
}
impl App {
    fn new(devices: Vec<Device>) -> Self {
        let mut state = TableState::default();
        if !devices.is_empty() {
            state.select(Some(0));
        }
        App {
            devices,
            state,
            status_msg: "Hazır. [↑/↓] Gezin | [C] Bağlan | [Q] Çıkış".to_string(),
        }
    }
}
