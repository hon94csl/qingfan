// Package bundle holds the font and BGM used when an automatic mix has no operator pick.
package bundle

import _ "embed"

// Lato is SIL OFL 1.1. LatoLicense is the notice that must travel with the font.
//
//go:embed Lato-Regular.ttf
var Lato []byte

//go:embed OFL.txt
var LatoLicense []byte

// BGM is a generated 196 Hz sine, two seconds, redistributed with the app.
//
//go:embed bgm_demo.wav
var BGM []byte
