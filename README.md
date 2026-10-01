# Konek

Konek is a Telegram bot daemon that connects your phone to Herdr and local AI coding agents such as OMP, Claude Code, Codex, and Pi. It requires no port forwarding, public IP, or VPN because it communicates via outbound HTTPS long polling.

## How it works

- Two-way communication over the Telegram Bot API. The bot runs on your local machine and polls Telegram servers directly. No inbound ports are opened on your local network.
- Direct Herdr socket connection. The bot watches agent lifecycle states (`idle`, `working`, `blocked`, `done`) and routes commands to the target pane.
- Approval handling. When an agent pauses at a confirmation dialog (e.g. bash execution or file editing), the bot provides interactive Approve, Reject, and Abort buttons in Telegram.
- Complete transcript extraction. For session-logging agents (such as OMP and Pi), the bot reads the full response from the session `.jsonl` file instead of truncating terminal scrollback.
- Automatic message chunking. Responses exceeding Telegram's 4,096-character limit are split cleanly by paragraphs and sent sequentially.
- Parallel execution. Multiple agents can run concurrently. Each turn maintains an independent progress card with real-time spinners and status updates.
- ID-based whitelist authorization. The bot drops messages from unauthorized Telegram accounts. Only configured user IDs can execute commands.

## System requirements

- Go 1.22 or later
- Herdr installed and running (`herdr status`)
- At least one supported coding agent (OMP, Claude Code, Codex, or Pi)

## Installation and setup

### 1. Obtain bot credentials

1. Open `@BotFather` on Telegram, send `/newbot`, and copy your bot token.
2. Open `@userinfobot` on Telegram to find your numeric account ID (e.g. `123456789`).

### 2. Configure environment variables

Copy the sample configuration file:

```bash
cp .env.example .env
```

Edit `.env`:

```env
TELEGRAM_BOT_TOKEN="your_token_from_botfather"
TELEGRAM_ALLOWED_USER_IDS="123456789"
HERDR_BIN_PATH="/Users/akbar/.local/bin/herdr"
DEFAULT_CWD="/Users/akbar/Code/projects"
```

To authorize multiple users, separate their IDs with commas in `TELEGRAM_ALLOWED_USER_IDS`.

### 3. Build and run

```bash
go build -o konek .
./konek
```

Send `/start` in Telegram to initialize the navigation keyboard.

## Commands and usage

### Interacting with agents

- Plain text message. Forwards the text as an instruction prompt to the active agent.
- `@name <prompt>`. Sends a prompt to a specific agent without switching your active target. The name can match a workspace label, project folder name, or pane ID (e.g. `@keep-silent fix auth validation` or `@w5:p1 run test suite`).
- Photo or screenshot attachment. Downloads the image locally and invokes the multimodal agent with the image path. Add your prompt in the photo caption.
- `/jobs`. Lists all running parallel tasks along with elapsed times and abort buttons.
- `/broadcast <prompt>`. Dispatches the prompt to all currently running agents at once.
- `/agents`. Opens an inline menu to inspect and select active coding agents.
- `/status`. Displays active agent details (project name, harness, lifecycle state, working directory).
- `/read [N]`. Without arguments, reads the complete latest response from the transcript. With an integer argument (e.g. `/read 50`), reads N lines from the active terminal scrollback.
- `/abort` or `/stop`. Immediately interrupts the active turn or running process using Escape and Ctrl+C.
- `/keys <key>`. Sends specific control keys to the agent terminal (e.g. `/keys enter`, `/keys esc`, `/keys y`, `/keys n`).

### Managing workspaces and panes

- `/workspaces`. Shows an interactive keyboard of all Herdr workspaces. Selecting a workspace lists its terminal panes.
- `/newworkspace <name> [folder]`. Creates a new workspace in Herdr. If the folder path is omitted, the directory defaults to `DEFAULT_CWD/<name>`. The bot prompts to launch OMP, Claude Code, Codex, or Pi inside the new pane.
- `/split [right|down]`. Splits the active terminal pane horizontally or vertically, then offers an agent launcher menu.
- `/sh <command>`. Runs a shell command on the host in the active project directory (e.g. `/sh git status`, `/sh git diff`).
- `/img <path>`. Uploads an image from the host machine directly to your Telegram chat (e.g. `/img screenshot.png` or an absolute path).
- `/menu`. Reopens the persistent bottom navigation keyboard if closed.

## Running in the background with macOS launchd

To run the bot as an automatic background service on macOS:

```bash
# Copy plist configuration to LaunchAgents
cp dev.konek.bot.plist ~/Library/LaunchAgents/

# Load and start the service
launchctl load ~/Library/LaunchAgents/dev.konek.bot.plist
```

To check service status:

```bash
launchctl list | grep dev.konek.bot
```

To stop the service:

```bash
launchctl unload ~/Library/LaunchAgents/dev.konek.bot.plist
```

Logs are written to the project directory:
- `konek.log` for standard output
- `konek.error.log` for standard error
