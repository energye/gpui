// Package world freezes the CPU-side entity math every 2.5D level feeds.
//
// Frozen 2026-09-15 (capability 10.1, P2, S25/W3): ID, NoEntity,
// Transform, IdentityTransform, LocalMatrix, Comp, World, NewWorld,
// Spawn, Despawn, Alive, Count, Spawned, SetParent, Parent, Children,
// SetTransform, Local, WorldOf, WorldMatrix, AddComp, RemoveComp,
// HasComp, Comps, Clear. Additive changes only.
//
// This package only computes numbers; it draws nothing and steps nothing.
// The caller spawns entities, hangs capabilities (sprite, collider,
// script) as comps, links parents, then reads world transforms and feeds
// them to the existing render draws. Only core numbers are used; render
// types convert once at the boundary with Vec2.ToRenderPoint and
// Mat2D.ToRenderMatrix. No Vec2, Color, or AssetID is redefined here.
//
// Identity: IDs start at 1 and never repeat inside one World; NoEntity
// (0) means no parent and never names a live entity. Spawned counts every
// birth so replays can prove no reuse; Count counts the living.
//
// Hierarchy (Godot node idea): each entity holds a local transform and at
// most one parent. WorldOf folds every ancestor from the root down:
// world scale is the component-wise product, world rotation is the plain
// sum, world position is the parent position plus the parent-rotated
// scaled local offset. WorldMatrix is the same fold as matrices
// (parent * local) and agrees with WorldOf inside 1e-9. Despawning a
// parent never deletes its children: they fall back to roots keeping
// their local transforms. Cycles and self-parenting are rejected, never
// guessed.
//
// Comps: Kind names the capability ("sprite", "collider", "script", ...)
// and must be non-empty; Ref carries the asset id and may stay empty for
// pure-logic comps. Duplicates are allowed (two scripts on one entity);
// RemoveComp drops the first match. Scene files (10.2) read this same
// shape: entity, comps, parent links, referenced asset ids.
//
// Errors: a missing entity is core NotFound, a bad number or bad link is
// core InvalidArg; bad writes store nothing. Nil receivers never panic:
// getters park at zero, writers report InvalidArg. Window intent: none,
// pure arithmetic, offscreen golden only.
//
// Frozen 2026-09-15 (capability 10.3, P2, S34/W4): LifeState,
// LifeActive, LifeSleeping, Scene, NewScene, Spawn, Sleep, Wake, State,
// IsActive, IsSleeping, Alive, Dispose, Count, ActiveCount,
// SleepingCount, Spawned, Clear, World. Additive changes only. Scene owns
// one World for storage; births run active, Sleep parks, Wake resumes,
// Dispose clears comps and flags. Window intent: none, pure arithmetic,
// offscreen golden only.
//
// Frozen 2026-09-15 (capability 10.2, P2, S33/W4): PrefabVersion,
// MaxPrefabNameLen, MaxPrefabComps, MaxPrefabBytes, Prefab, NewPrefab,
// Name, Version, Local, SetLocal, AddComp, Comps, Encode, Equal,
// Instantiate, ParsePrefab, LoadPrefab, SavePrefab, SceneFileVersion,
// MaxSceneNameLen, MaxSceneEntities, MaxSceneComps, MaxSceneBytes,
// SceneNode, SceneFile, NewSceneFile, Name, Version, Count, Entities,
// AddEntity, ParseScene, LoadScene, SaveScene, Open. Additive changes only. Prefab
// is one reusable entity template (name, local, comps); SceneFile is one
// level on disk (name, version, entities with parent links, comps,
// referenced asset ids). Parse/Load read scene JSON, Encode/Save write
// canonical bytes, Instantiate stamps a prefab into a World, Open builds
// a live Scene (every birth active) plus the filed-name to live-id
// index. Intentional deviation: the plan table names Scene{Load,Save};
// here SceneFile+LoadScene/SaveScene/Open because S34 already owns the
// live Scene name in this package and a second Scene would not compile.
// File shape frozen: version "1.0", entity, comps, parent links,
// referenced asset ids. Window intent: game_world--case=open; pure file
// math, offscreen golden only (round-trip byte-identical).
package world
