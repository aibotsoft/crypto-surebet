package signals

import (
	"os"
	"os/signal"
	"syscall"
)

// SetupSignalHandler registered for SIGTERM and SIGINT. A stop channel is returned
// which is closed on one of these signals. If a second signal is caught, the program
// is terminated with exit code 1.

var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT}

func SetupSignalHandler() (stopCh <-chan os.Signal) {
	//c := make(chan os.Signal)
	c := make(chan os.Signal, 1)
	//signal.Notify(c)
	signal.Notify(c, shutdownSignals...)
	return c
}
