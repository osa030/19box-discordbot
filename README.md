# 19box Discord Bot

A Discord bot client for the **19box Jukebox** server. It notifies Discord users of session events and allows them to request tracks via slash commands.

## Features

- **Real-time Notifications**: Announces session starts, ends, and track changes in a Discord forum thread.
- **Track Requests**: Allows users to request Spotify tracks using the `/req` slash command.
- **Automated Thread Management**: Automatically creates and manages forum threads for each session.
- **Context-Aware**: Support for timeouts and graceful shutdown for improved reliability.

## Prerequisites

- Go 1.25 or later
- [Buf](https://buf.build/) (for Protocol Buffer generation)
- A Discord Bot Token
- A running [19box Jukebox Server](https://github.com/osa030/19box)

## Configuration

The bot is configured primarily through environment variables or command-line flags.

### Environment Variables

You can set these in a `.env` file or exported in your shell:

| Variable | Description | Requirement |
|----------|-------------|-------------|
| `DISCORD_BOT_TOKEN` | Your Discord bot token | **Required** |
| `DISCORD_GUILD_ID` | The ID of the Discord server (Guild) | **Required** |
| `DISCORD_FORUM_ID` | The ID of the forum channel where sessions will be posted | **Required** |
| `JUKEBOX_SERVER_URL` | The address of the Jukebox server (Default: `http://localhost:8080`) | Optional |
| `VERBOSE` | Set to `true` for debug logging | Optional |
| `LOGFILE` | Path to log file (Default: stdout) | Optional |

### Command-line Flags

Flags take precedence over environment variables:

- `--token`: Discord bot token
- `--guild-id`: Discord guild ID
- `--forum-id`: Discord forum ID
- `--server`: Jukebox server address
- `--verbose`: Enable debug logging
- `--logfile`: Path to log file

## Installation & Usage

1. **Clone the repository** (with submodule):
   ```bash
   git clone --recursive https://github.com/osa030/19box-discordbot.git
   cd 19box-discordbot
   ```

   If you already cloned without `--recursive`, initialize the submodule:
   ```bash
   git submodule update --init --recursive
   ```

2. **Setup environment**:
   Create a `.env` file with your Discord credentials.

3. **Build and Run**:
   ```bash
   ./build.sh
   ./bin/19box-discordbot start
   ```

   Alternatively, use the provided helper script:
   ```bash
   ./19box-discordbot.sh start
   ```

   > **Note**: Make sure the 19box server is running before starting the bot.

## Discord Commands

- `/req [url]`: Request a track by its Spotify URL.

## Project Structure

- `cmd/discordbot/`: Entry point and initialization logic.
- `internal/app/bot/`:
    - `bot.go`: Core lifecycle and notification management.
    - `command.go`: Slash command definitions and handlers.
    - `ui.go`: Message templates and Embed construction.
    - `config.go`: Configuration structures and validation.
- `internal/jukebox/`: Connect client for the 19box server.
- `internal/logger/`: Structured logging utility.
- `internal/timezone/`: Platform-specific timezone initialization.
- `internal/gen/`: Generated code from Protobuf definitions.
- `proto/`: Git submodule containing Protocol Buffer definitions from [19box](https://github.com/osa030/19box).

## Development

### Generating Protocol Buffers

The `proto/` directory is a git submodule pointing to the main 19box repository.

1. **Update the submodule** (to get latest proto definitions):
   ```bash
   git submodule update --remote proto
   ```

2. **Generate Go code**:
   ```bash
   buf generate proto
   ```

   This generates Connect RPC client code in `internal/gen/`.

## License

MIT
