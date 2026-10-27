package main

import "fmt"

func buildScene(name string, gate bool) (*sceneSpec, error) {
	switch name {
	case sceneSpin:
		return buildSpinScene(), nil
	case sceneList:
		return buildListScene(gate)
	case sceneText:
		return buildTextScene()
	case sceneGodot:
		return buildGodotScene(gate)
	default:
		return nil, fmt.Errorf("no builder for scene %q", name)
	}
}
