// Package scene holds retained scene types for L1 (Layer tree, FramePacket, Picture).
package scene

// Picture is a retained draw-ops handle (P2+).
// Zero value means "no retained picture".
type Picture struct {
	// ID is stable identity for cache keys (optional).
	ID uint64
	// Valid is false when the picture must be re-recorded.
	Valid bool
}
