# Barry - Discord Bot for Fly Machines

A Discord bot that starts Fly.io machines and monitors their health status.

## Features

- `/start-server` slash command to start a Fly machine
- Automatic health check polling every 30 seconds
- Discord notifications when the server is ready

## Prerequisites

- Go 1.21 or later
- A Discord bot token
- A Fly.io API token
- A Fly.io app with a machine configured

## Setup

1. Clone this repository

2. Install dependencies:
```bash
go mod download
```

3. Copy the example environment file:
```bash
cp .env.example .env
```

4. Edit `.env` and fill in your configuration:
   - `DISCORD_BOT_TOKEN`: Your Discord bot token (get from https://discord.com/developers/applications)
   - `FLY_API_TOKEN`: Your Fly.io API token (get from `fly auth token`)
   - `FLY_APP_NAME`: Your Fly.io app name
   - `FLY_MACHINE_ID`: The machine ID to start

5. Build the bot:
```bash
go build -o barry
```

6. Run the bot:
```bash
./barry
```

## Discord Bot Setup

1. Go to https://discord.com/developers/applications
2. Create a new application
3. Go to the "Bot" section
4. Create a bot and copy the token
5. Enable "Message Content Intent" in the Bot settings
6. Go to "OAuth2" > "URL Generator"
7. Select `bot` scope and `applications.commands` scope
8. Copy the generated URL and open it in your browser to invite the bot to your server

## Usage

Once the bot is running and invited to your Discord server, use the `/start-server` command in any channel where the bot has access.

The bot will:
1. Acknowledge the command immediately
2. Start the Fly machine
3. Poll the machine status via Fly Machines API every 30 seconds
4. Check configured health checks from the machine configuration
5. Notify the channel when the server is healthy and ready

## Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `DISCORD_BOT_TOKEN` | Discord bot token | Yes |
| `FLY_API_TOKEN` | Fly.io API token | Yes |
| `FLY_APP_NAME` | Fly.io application name | Yes |
| `FLY_MACHINE_ID` | Machine ID to start | Yes |

## License

MIT

