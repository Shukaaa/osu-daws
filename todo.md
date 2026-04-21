# Better Workspaces (Planned Release: 0.3.0)
- ✅ Last opened workspace
- ✅ Search Workspace List
- Archive Workspaces to ignore them in the list and show them in a separate list of archived workspaces
- ✅ Export Workspaces as zip files that can be shared with other people. This zip file would include the workspace settings, the DAW template with the hitsound structure, and the generated diff export.

# UX Improvements (Planned Release: 0.3.0)
- Better Feedback for successful actions (e.g. "Hitsound diff generated successfully!" notification after generating the diff)

# Better Section Management (Planned Release: 0.4.0)
- User is able to Rename Sections for better organization (e.g. "Intro", "Verse", "Chorus", etc.)
- Enable or Disable sections to easily test different versions of the hitsound structure in-game without having to delete or move notes around in the DAW

# Settings-Section (Planned Release: 0.5.0)
- Add a helper to install the FL Studio Script via osu!daws
- Change Workspace Folder location and migration setting to copy existing workspaces to the new location instead of just changing the location for new workspaces
- Export Settings:
    - Default hitobject position optional
    - Default generated diff name suffix (e.g. HS, Hitsounds, osu!daw's HS)
    - Default gamemode for HS diff: osu!standard or Catch
- Automatically open last workspace on app start option

# Better Exports (Planned Release: 1.0.0)
- New Button to directly open the generated diff in osu! Editor after generation
- Versionate Hitsound Diff exports v1, v2, ...
- Statistic after generation: number of hitsounds, number of custom sample sets, etc.
- Better Volume Distribution Option
  - One of the problems is, that volume percentages can differ ~2-3% because in FL you have to klick the exact same pixel which can sometimes be hard.
  - A better option would be to have a "Volume Step Size" setting in osu!daws which defines the step size for volume changes.
  - For example, if you set it to 5%, then any velocity that falls within a 5% range would be rounded to the nearest step. So if you have a velocity that corresponds to 72%, and your step size is 5%, it would round to either 70% or 75% depending on which one is closer.
  - This way, you can ensure more consistent volume levels without having to worry about pixel-perfect clicks in FL Studio.
