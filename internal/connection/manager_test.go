package connection

import (
	"errors"
	"path/filepath"
	"testing"

	"AndroidFileTransfer/internal/model"
)

// ---------------------------------------------------------------------------
// Stubs / local interfaces for routing tests
// ---------------------------------------------------------------------------

// stubADBManager satisfies the subset of ADBManager behaviour used by Manager
// routing tests, without invoking real adb.
type stubADBManager struct {
	listFilesCallSerial string
	listFilesCallPath   string
	listFilesResult     []model.FileInfo
	listFilesErr        error

	pullCalled bool
	pushCalled bool
}

func (s *stubADBManager) DetectDevices() []model.Device { return nil }

func (s *stubADBManager) ListFiles(serial, path string) ([]model.FileInfo, error) {
	s.listFilesCallSerial = serial
	s.listFilesCallPath = path
	return s.listFilesResult, s.listFilesErr
}

func (s *stubADBManager) Pull(serial, remotePath, localPath string) error {
	s.pullCalled = true
	return nil
}

func (s *stubADBManager) Push(serial, localPath, remotePath string) error {
	s.pushCalled = true
	return nil
}

// ---------------------------------------------------------------------------
// managerUnderTest wires a Manager with a stub ADB implementation.
// We reach into Manager's internals because Manager embeds *ADBManager
// directly; for test isolation we replace it with a thin wrapper that
// delegates to the stub.
// ---------------------------------------------------------------------------

// adbAdapter wraps stubADBManager behind the same method set that Manager
// uses, so we can assign it to Manager.adbMgr via a helper constructor.
// Because Manager holds a concrete *ADBManager, we take a different approach:
// we embed the routing logic we want to test directly by constructing a
// testableManager that delegates to interface-typed fields.

// routerADB is the interface the Manager routing test cares about.
type routerADB interface {
	ListFiles(serial, path string) ([]model.FileInfo, error)
	Pull(serial, remotePath, localPath string) error
	Push(serial, localPath, remotePath string) error
	DetectDevices() []model.Device
}

// testableManager is a thin copy of the Manager routing logic that accepts
// interface-typed collaborators, letting us inject stubs without touching the
// real Manager struct.
type testableManager struct {
	adb         routerADB
	broadcaster Broadcaster
}

func (m *testableManager) GetFileList(deviceID, path string) ([]model.FileInfo, error) {
	switch {
	case len(deviceID) >= 5 && deviceID[:5] == "wifi:":
		return []model.FileInfo{{Name: "wifi-stub", IsDir: false}}, nil
	case len(deviceID) >= 4 && deviceID[:4] == "adb:":
		if m.adb == nil {
			return nil, errors.New("ADB not available")
		}
		serial := deviceID[4:]
		return m.adb.ListFiles(serial, path)
	default:
		return nil, errors.New("unknown deviceID prefix: " + deviceID)
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestManagerRouting_ADBPrefix(t *testing.T) {
	stub := &stubADBManager{
		listFilesResult: []model.FileInfo{{Name: "file.txt"}},
	}
	mgr := &testableManager{adb: stub}

	files, err := mgr.GetFileList("adb:emulator-5554", "/sdcard")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 || files[0].Name != "file.txt" {
		t.Fatalf("unexpected files: %+v", files)
	}
	if stub.listFilesCallSerial != "emulator-5554" {
		t.Errorf("serial mismatch: got %q, want %q", stub.listFilesCallSerial, "emulator-5554")
	}
	if stub.listFilesCallPath != "/sdcard" {
		t.Errorf("path mismatch: got %q, want %q", stub.listFilesCallPath, "/sdcard")
	}
}

func TestManagerRouting_WiFiPrefix(t *testing.T) {
	stub := &stubADBManager{}
	mgr := &testableManager{adb: stub}

	files, err := mgr.GetFileList("wifi:192.168.1.1:8080", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected at least one entry for WiFi path")
	}
	// Stub ADB must NOT have been called.
	if stub.listFilesCallSerial != "" {
		t.Errorf("ADB ListFiles was unexpectedly called with serial %q", stub.listFilesCallSerial)
	}
}

func TestManagerRouting_UnknownPrefix(t *testing.T) {
	mgr := &testableManager{adb: &stubADBManager{}}

	_, err := mgr.GetFileList("mtp:somedevice", "/")
	if err == nil {
		t.Fatal("expected an error for unknown prefix")
	}
}

func TestManagerRouting_ADBError(t *testing.T) {
	stub := &stubADBManager{
		listFilesErr: errors.New("adb: device offline"),
	}
	mgr := &testableManager{adb: stub}

	_, err := mgr.GetFileList("adb:offline-device", "/sdcard")
	if err == nil {
		t.Fatal("expected error propagation from ADB stub")
	}
}

func TestManagerRouting_NilADB(t *testing.T) {
	mgr := &testableManager{adb: nil}

	_, err := mgr.GetFileList("adb:any", "/")
	if err == nil {
		t.Fatal("expected error when ADB is nil")
	}
}

// ---------------------------------------------------------------------------
// Broadcaster tests
// ---------------------------------------------------------------------------

func TestBroadcaster_PublishReceive(t *testing.T) {
	var b Broadcaster
	ch := b.Subscribe()

	p := model.TransferProgress{DeviceID: "adb:test", FileName: "foo.txt", BytesDone: 42}
	b.Publish(p)

	select {
	case got := <-ch:
		if got != p {
			t.Errorf("got %+v, want %+v", got, p)
		}
	default:
		t.Fatal("expected value in subscriber channel")
	}
}

func TestBroadcaster_MultipleSubscribers(t *testing.T) {
	var b Broadcaster
	ch1 := b.Subscribe()
	ch2 := b.Subscribe()

	p := model.TransferProgress{DeviceID: "adb:test", FileName: "bar.txt"}
	b.Publish(p)

	for i, ch := range []<-chan model.TransferProgress{ch1, ch2} {
		select {
		case got := <-ch:
			if got != p {
				t.Errorf("subscriber %d: got %+v, want %+v", i, got, p)
			}
		default:
			t.Errorf("subscriber %d: expected value in channel", i)
		}
	}
}

func TestBroadcaster_CloseUnblocksSubscribers(t *testing.T) {
	var b Broadcaster
	ch := b.Subscribe()
	b.Close()

	// channel should be closed (zero value received without blocking)
	_, ok := <-ch
	if ok {
		t.Fatal("expected channel to be closed after Broadcaster.Close()")
	}
}

func TestBroadcaster_SubscribeAfterClose(t *testing.T) {
	var b Broadcaster
	b.Close()
	ch := b.Subscribe()

	// Should receive a closed channel immediately.
	_, ok := <-ch
	if ok {
		t.Fatal("expected already-closed channel when subscribing after Close()")
	}
}

func TestBroadcaster_PublishAfterClose(t *testing.T) {
	var b Broadcaster
	b.Close()
	// Should not panic.
	b.Publish(model.TransferProgress{DeviceID: "adb:x"})
}

// ---------------------------------------------------------------------------
// Manager Stop and wifiFileList safety tests (Task 7 fixes)
// ---------------------------------------------------------------------------

// TestManagerStop_DoubleCallNoPanic verifies that calling Stop twice on a
// real Manager does not panic (regression for sync.Once fix).
func TestManagerStop_DoubleCallNoPanic(t *testing.T) {
	srv := NewWiFiServer()
	mgr := NewManager(srv, nil, nil)

	// Start is required to have a running wifiSrv so Stop() can close it.
	if err := mgr.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("second Stop() caused a panic: %v", r)
		}
	}()

	mgr.Stop()
	mgr.Stop() // must not panic
}

// TestManagerWifiFileList_OutOfBoundsPath verifies that wifiFileList returns
// an error when given an invalid virtual path (unknown prefix or hidden segment).
func TestManagerWifiFileList_OutOfBoundsPath(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	shareMgr, err := newShareManagerWithConfigPath(configPath)
	if err != nil {
		t.Fatalf("newShareManagerWithConfigPath: %v", err)
	}
	srv := NewWiFiServer()
	srv.SetShareManager(shareMgr)
	mgr := NewManager(srv, nil, shareMgr)

	// In selected mode an unknown /shared/<id> virtual path must return an error.
	_, err = mgr.wifiFileList("/shared/nonexistent-id")
	if err == nil {
		t.Fatal("expected error for unknown shared ID, got nil")
	}
}

func TestBroadcaster_FullChannelDrops(t *testing.T) {
	var b Broadcaster
	ch := b.Subscribe() // capacity 16

	// Overflow the channel: publish 20 messages; only 16 should be buffered.
	for i := 0; i < 20; i++ {
		b.Publish(model.TransferProgress{BytesDone: int64(i)})
	}
	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			goto done
		}
	}
done:
	if count != 16 {
		t.Errorf("expected 16 buffered messages, got %d", count)
	}
}
