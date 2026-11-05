
## origin/swarm

Has:

- DeleteSecret: main still has no secret deletion, but it does have DeleteConfig at swarm/data.go:165, and it's a four-line method. Porting the old one is no easier than writing it fresh when you need it. The old test and fixture target the pre-rework client, so they wouldn't apply anyway.
- Label note: main still discovers versions by listing every secret and prefix-matching on {base}_v{N}, so the problem the note describes still exists. But the note itself is just a paragraph of idea, not code. If it's worth keeping, a line in a todo is all it needs.

Run these down and then get rid of the stale branch please.
