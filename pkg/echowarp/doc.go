// Package echowarp provides the core EchoWarp audio streaming functionality.
//
// EchoWarp is a real-time audio streaming application that captures audio
// from one computer and plays it on another over the network.
//
// The main entry point is the Node type, which manages the complete lifecycle
// of an audio streaming session:
//
//	node := echowarp.NewNode(cfg)
//	if err := node.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//	defer node.Stop()
package echowarp
