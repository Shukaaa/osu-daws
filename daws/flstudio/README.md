# FL Studio Instructions

This guide explains the FL Studio side of the `osu!daws` workflow.

## 1. Install the script

Copy the script from:

```text
/daws/FLStudio/scripts
```

into your FL Studio Piano Roll Scripts folder.

Example:

```text
Documents/Image-Line/FL Studio/Settings/Piano roll scripts
```

After that, restart FL Studio if it was already open.

## 2. Open the project through osu!daws

Do not open the FL project manually.

Open FL Studio through `osu!daws` so the correct workspace project/template is loaded automatically.

## 3. Build hitsounds in the FPC Piano Roll

Create your hitsounds in the **Piano Roll of the FPC plugin**.

The note names shown in the Piano Roll tell you which hitsound each note represents.

Use those notes to place your hitsounds.

## 4. Control volume with note velocity

Use the **note velocity** lane at the bottom of the Piano Roll to control hitsound volume.

Higher velocity = higher volume
Lower velocity = lower volume

## 5. Control custom sample sets with MIDI color

Use **MIDI Color** to define the custom sample set index.

* Color `1` to `16` = custom sample set index
* The selected color applies to the note
* Notes with different colors represent different custom sample sets

## 6. Important rules

### One custom sample set per position

Do **not** place notes on the same position with different custom sample set colors.

### Use the mapped notes only

Only use the intended hitsound notes from the template/FPC mapping. Otherwise, the compiler won't know how to interpret them.

## 7. Export the SourceMap

When your hitsounds are finished:

1. Open the Piano Roll script menu
2. Run the export script
3. Copy the **entire output**
4. Go back to `osu!daws`
5. Paste the copied output into a segment

That copied output is your **SourceMap**.

## 8. Continue in osu!daws

After pasting the SourceMap into a segment in `osu!daws`:

* set the segment start time
* generate the hitsound diff
* review the preview/export result

## Notes

This workflow is meant to keep FL Studio focused on authoring the hitsound structure, while `osu!daws` handles conversion and final map generation.

