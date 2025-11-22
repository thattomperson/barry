# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Barry is a Discord bot that starts Fly.io machines and monitors their health status. It provides a `/start-server` slash command that triggers machine startup and polls for health check completion.

## Build and Run Commands

```bash
# Install dependencies
go mod download

# Build
go build -o barry

# Run locally (requires environment variables)
./barry

# Build Docker image
docker build -t barry .
```

## Environment Variables

The bot requires these environment variables (note: code uses `MC_` prefix, which differs from README):
- `DISCORD_BOT_TOKEN` - Discord bot token
- `MC_FLY_API_TOKEN` - Fly.io API token
- `MC_FLY_APP_NAME` - Fly.io application name
- `MC_FLY_MACHINE_ID` - Machine ID to start

## Architecture

Two-file Go application:
- `main.go` - Discord bot setup, slash command registration, interaction handling
- `machine.go` - Fly.io Machines API integration (start machine, health check polling)

### Key Flow
1. User triggers `/start-server` command
2. Bot immediately acknowledges (Discord requires response within 3 seconds)
3. Goroutine starts the Fly machine via API
4. Polls machine status every 30 seconds until healthy
5. Notifies channel when server is ready

### Health Check Logic
- If machine has configured health checks: waits for all checks to report "passing"
- If no health checks configured: considers machine ready when state is "started"

## Deployment

Deployed to Fly.io using `fly.toml` (Sydney region, shared-cpu-1x VM). Uses multi-stage Docker build with Go builder and Debian runtime.
