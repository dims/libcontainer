// +build linux

package systemd

import (
	"context"
	"sync"

	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/sirupsen/logrus"
)

type dbusConnManager struct {
	conn     *systemdDbus.Conn
	rootless bool
	sync.RWMutex
}

// newDbusConnManager initializes systemd dbus connection
func newDbusConnManager(rootless bool) *dbusConnManager {
	return &dbusConnManager{
		rootless: rootless,
	}
}

// getConnection lazy initializes systemd dbus connection and returns it
func (d *dbusConnManager) getConnection() (*systemdDbus.Conn, error) {
	// In the case where d.conn != nil
	// Use the read lock the first time to ensure
	// that Conn can be acquired at the same time.
	d.RLock()
	if conn := d.conn; conn != nil {
		d.RUnlock()
		return conn, nil
	}
	d.RUnlock()

	// In the case where d.conn == nil
	// Use write lock to ensure that only one
	// will be created
	d.Lock()
	defer d.Unlock()
	if conn := d.conn; conn != nil {
		return conn, nil
	}

	conn, err := d.newConnection()
	if err != nil {
		return nil, err
	}
	d.conn = conn
	return conn, nil
}

func (d *dbusConnManager) newConnection() (*systemdDbus.Conn, error) {
	if d.rootless {
		return newUserSystemdDbus()
	}
	return systemdDbus.NewWithContext(context.TODO())
}

// resetConnection reset to initial state
func (d *dbusConnManager) resetConnection(conn *systemdDbus.Conn) {
	d.Lock()
	defer d.Unlock()
	if d.conn != nil && d.conn == conn {
		d.conn.Close()
		d.conn = nil
	}
}

// checkAndReconnect check if the connection is disconnected and reconnect
func (d *dbusConnManager) checkAndReconnect(conn *systemdDbus.Conn, err error) {
	if !isDbusClosed(err) {
		return
	}
	d.resetConnection(conn)

	// Try to reconnect
	_, err = d.getConnection()
	if err != nil {
		logrus.Warnf("Dbus disconnected and failed to reconnect: %s", err)
	}
}
