# osu!daws

`osu!daws` is a local tool for creating osu! hitsound diffs with the help of a DAW.

Instead of placing everything by hand inside the osu! editor, you can build your hitsound structure in a supported DAW, bring it into `osu!daws`, and generate a dedicated hitsound difficulty from it.

## What it does

`osu!daws` helps you:

- turn your DAW hitsound project into an osu! hitsound diff
- keep your project work organized in workspaces
- reuse your project settings without setting everything up again every time
- generate a clean hs diff that can be copied into your osu! map folder when needed

## How the workflow looks

At a high level, the process is simple:

1. Create or open a workspace in `osu!daws`
2. Use a supported DAW template/workflow to build your hitsound structure
3. Create Hitsounds in your DAW with the provided DAW Template
4. Bring that data into the app (Refer to the DAW-specific instructions for how to do this)
5. Select the reference `.osu` difficulty (Copies the timing and metadata from that diff)
6. Generate the hitsound diff
7. Copy the result into your osu! project folder if you want to test it in-game

## Workspaces

Each map project lives in its own workspace.

A workspace stores things like:

- your project settings
- reference map information
- DAW-related template files
- generated exports

This keeps everything organized and avoids putting project-only files directly into your beatmap folder.

## DAW support

`osu!daws` is designed to support different DAWs through templates and DAW-specific workflows.

For DAW-specific setup and usage, see:

- `/daws/[daw]/README.md`

Example:

- `/daws/flstudio/README.md`

> Curently we only support FL Studio, because that's the only DAW I work with :(
> 
> Feel free to contribute templates for other DAWs if you want to see them supported!

## Goal of the project

The goal of `osu!daws` is to make hitsound work faster, cleaner, and more comfortable for people who prefer building music in a DAW instead of doing everything manually inside osu!.

## Status

This project is under active development, and the workflow and features may continue to improve over time.