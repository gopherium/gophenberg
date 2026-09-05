---
title: Declare content from a plugin
description: How a plugin brings its own content types, field groups and fields to every site it is compiled into.
---

A plugin that needs a content type should not ask the site owner to
build one by hand. Declare it in code, and every site the plugin is
compiled into gets it at startup.

## The one method

Implement `DeclareTypes` and the host hands you a registrar before
anything serves:

```go
func (p *Plugin) DeclareTypes(ctx context.Context, types sdk.TypeRegistrar) error {
	if err := types.DeclareType(ctx, sdk.TypeDeclaration{
		Key: "event", SingularLabel: "Event", PluralLabel: "Events", RouteWord: "events",
	}); err != nil {
		return err
	}
	return types.DeclareGroup(ctx, sdk.GroupDeclaration{
		Key:      "event-details",
		Title:    "Event details",
		Location: [][]sdk.Rule{{{Source: "content_type", Operator: "==", Value: "event"}}},
		Fields: []sdk.FieldDeclaration{
			{Key: "venue", Label: "Venue", Kind: "text", Required: true},
			{Key: "ticketed", Label: "Ticketed", Kind: "boolean"},
			{Key: "price", Label: "Price", Kind: "number", Conditions: [][]sdk.Rule{
				{{Source: "ticketed", Operator: "==", Value: "true"}},
			}},
			{Key: "schedule", Label: "Schedule", Kind: "section", Fields: []sdk.FieldDeclaration{
				{Key: "starts-at", Label: "Starts at", Kind: "date"},
			}},
		},
	})
}
```

`Conditions` says when a field shows, reading the fields standing
beside it. Declare them in any order you like, the host writes the
fields first and their conditions afterwards. `Listed: true` marks a
field for the content list, which nothing reads yet.

Keys follow the same shape everywhere in Gophenberg. Start with a
lowercase letter, then lowercase letters, numbers and hyphens.

## Declaring runs on every start

The registrar is not a one time installer. It runs at every startup,
and it is written to be safe to repeat:

- A definition that is not there yet is created.
- One that is there and matches is left alone.
- A changed label, required flag, setting or location is carried
  onto the stored one.

So the way to change a field is to edit the declaration and restart.
There is no migration to write and no version to bump.

## Two changes are refused

Changing a field's **Kind** and changing a type's **Route word**
both strand content that is already stored. The host refuses them
and the site will not start, which is loud on purpose.

To change either one, declare a new key beside the old one, move
what you need, and stop declaring the old key.

## What the site owns and what you own

A definition your plugin declares belongs to your plugin. The admin
shows it with a badge naming the plugin. Nobody can edit or delete
it from the screens, though they can turn it off, and they can move
it in the order.

If the site already holds a type or group under a key you declare,
your declaration is skipped. The site's own definition wins, always.
The start log says which keys were skipped, and the **Field Groups**
screen names the clash so the site owner can see it.

That rule is why you should pick keys nobody else would: prefix
them with your plugin's own name if you expect company.

## When a plugin stops declaring

Stop declaring a definition and the rows stay where they are, with
your plugin's name still on them. The site owner sees them on the
**Field Groups** screen as coming from a plugin that no longer
declares them, and gets one button, **Adopt**, which hands the
definition to the site.

Nothing is deleted behind anyone's back. A definition that holds
content keeps holding it until a person decides otherwise.

## Reading the values back

Field values reach a plugin through the read seam:

```go
items, err := deps.Content.ListPublished(ctx, "event", 10)
venue := items[0].Fields["venue"]
```

`Fields` holds the values keyed by field key, exactly as stored.
They are data, not markup, so escape them before serving them as
HTML. The [plugin SDK](/extending/the-plugin-sdk/) page covers the
rest of what `Deps` gives you.
