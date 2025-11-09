package server

import (
	"context"
	"testing"
	"time"
)

// TestAutoExitWithoutLinger tests that server exits after 5 seconds with no connections
func TestAutoExitWithoutLinger(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create server with linger=false
	srv := &Server{
		ctx:            ctx,
		cancel:         cancel,
		connections:    make(map[*WSConnection]bool),
		peerConnection: make(map[string]*WSConnection),
		linger:         false,
	}

	// Simulate last connection closing
	srv.startExitTimer()

	// Wait for context to be cancelled (should happen in ~5 seconds)
	select {
	case <-srv.ctx.Done():
		// Success - context was cancelled
	case <-time.After(6 * time.Second):
		t.Fatal("Server did not auto-exit within 6 seconds")
	}
}

// TestAutoExitCancellation tests that exit timer is cancelled when new connection arrives
func TestAutoExitCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create server with linger=false
	srv := &Server{
		ctx:            ctx,
		cancel:         cancel,
		connections:    make(map[*WSConnection]bool),
		peerConnection: make(map[string]*WSConnection),
		linger:         false,
	}

	// Start exit timer
	srv.startExitTimer()

	// Wait 2 seconds
	time.Sleep(2 * time.Second)

	// Cancel exit timer (simulating new connection)
	srv.cancelExitTimer()

	// Wait another 4 seconds - context should NOT be cancelled
	select {
	case <-srv.ctx.Done():
		t.Fatal("Server auto-exited even though timer was cancelled")
	case <-time.After(4 * time.Second):
		// Success - server stayed alive
	}
}

// TestLingerMode tests that server with linger=true never auto-exits
func TestLingerMode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create server with linger=true
	srv := &Server{
		ctx:            ctx,
		cancel:         cancel,
		connections:    make(map[*WSConnection]bool),
		peerConnection: make(map[string]*WSConnection),
		linger:         true,
	}

	// Simulate connection closing - should NOT start timer
	if len(srv.connections) == 0 && !srv.linger {
		srv.startExitTimer()
	}

	// Wait 6 seconds - context should NOT be cancelled
	select {
	case <-srv.ctx.Done():
		t.Fatal("Server in linger mode should not auto-exit")
	case <-time.After(6 * time.Second):
		// Success - server stayed alive
	}
}

// TestExitTimerRestarts tests that each disconnection restarts the timer
func TestExitTimerRestarts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create server with linger=false
	srv := &Server{
		ctx:            ctx,
		cancel:         cancel,
		connections:    make(map[*WSConnection]bool),
		peerConnection: make(map[string]*WSConnection),
		linger:         false,
	}

	// First disconnection - start timer
	srv.startExitTimer()

	// Wait 4 seconds
	time.Sleep(4 * time.Second)

	// Cancel and restart timer (simulating reconnect then disconnect)
	srv.cancelExitTimer()
	srv.startExitTimer()

	// Wait 4 more seconds (total 8 seconds from first start)
	// Context should NOT be cancelled yet (only 4 seconds since restart)
	select {
	case <-srv.ctx.Done():
		t.Fatal("Server exited before 5 seconds elapsed since timer restart")
	case <-time.After(4 * time.Second):
		// Success - timer was properly restarted
	}

	// Wait 2 more seconds - NOW context should be cancelled (5 seconds since restart)
	select {
	case <-srv.ctx.Done():
		// Success - context cancelled after 5 seconds from restart
	case <-time.After(2 * time.Second):
		t.Fatal("Server did not exit after 5 seconds since timer restart")
	}
}

// TestCancelExitTimerMultipleTimes tests that cancelling timer multiple times is safe
func TestCancelExitTimerMultipleTimes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := &Server{
		ctx:            ctx,
		cancel:         cancel,
		connections:    make(map[*WSConnection]bool),
		peerConnection: make(map[string]*WSConnection),
		linger:         false,
	}

	// Start timer
	srv.startExitTimer()

	// Cancel multiple times - should not panic
	srv.cancelExitTimer()
	srv.cancelExitTimer()
	srv.cancelExitTimer()

	// Verify context is not cancelled
	select {
	case <-srv.ctx.Done():
		t.Fatal("Context was cancelled unexpectedly")
	case <-time.After(1 * time.Second):
		// Success - no panic, context not cancelled
	}
}

// TestStopCancelsTimer tests that Stop() cancels the exit timer
func TestStopCancelsTimer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := &Server{
		ctx:            ctx,
		cancel:         cancel,
		connections:    make(map[*WSConnection]bool),
		peerConnection: make(map[string]*WSConnection),
		linger:         false,
	}

	// Start timer
	srv.startExitTimer()

	// Stop server (should cancel timer)
	srv.Stop()

	// Wait - context should be cancelled but timer should be cleaned up
	time.Sleep(6 * time.Second)

	// Verify timer is nil (cleaned up)
	srv.exitTimerMu.Lock()
	timerIsNil := srv.exitTimer == nil
	srv.exitTimerMu.Unlock()

	if !timerIsNil {
		t.Fatal("Exit timer was not cleaned up by Stop()")
	}
}
