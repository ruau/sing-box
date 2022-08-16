package main

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"

	"github.com/stretchr/testify/require"
)

func startInstance(t *testing.T, options option.Options) {
	var instance *box.Box
	var err error
	for retry := 0; retry < 3; retry++ {
		instance, err = box.New(context.Background(), options)
		require.NoError(t, err)
		err = instance.Start()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		break
	}
	require.NoError(t, err)
	t.Cleanup(func() {
		instance.Close()
	})
}

func testTCP(t *testing.T, clientPort uint16, testPort uint16) {
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", clientPort), socks.Version5, "", "")
	dialTCP := func() (net.Conn, error) {
		return dialer.DialContext(context.Background(), "tcp", M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	}
	require.NoError(t, testPingPongWithConn(t, testPort, dialTCP))
}

func testSuit(t *testing.T, clientPort uint16, testPort uint16) {
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", clientPort), socks.Version5, "", "")
	dialTCP := func() (net.Conn, error) {
		return dialer.DialContext(context.Background(), "tcp", M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	}
	dialUDP := func() (net.PacketConn, error) {
		return dialer.ListenPacket(context.Background(), M.ParseSocksaddrHostPort("127.0.0.1", testPort))
	}
	/*t.Run("tcp", func(t *testing.T) {
		t.Parallel()
		var err error
		for retry := 0; retry < 3; retry++ {
			err = testLargeDataWithConn(t, testPort, dialTCP)
			if err == nil {
				break
			}
		}
		require.NoError(t, err)
	})
	t.Run("udp", func(t *testing.T) {
		t.Parallel()
		var err error
		for retry := 0; retry < 3; retry++ {
			err = testLargeDataWithPacketConn(t, testPort, dialUDP)
			if err == nil {
				break
			}
		}
		require.NoError(t, err)
	})*/
	//require.NoError(t, testPingPongWithConn(t, testPort, dialTCP))
	//require.NoError(t, testPingPongWithPacketConn(t, testPort, dialUDP))
	require.NoError(t, testLargeDataWithConn(t, testPort, dialTCP))
	require.NoError(t, testLargeDataWithPacketConn(t, testPort, dialUDP))

	// require.NoError(t, testPacketConnTimeout(t, dialUDP))
}

func testSuitWg(t *testing.T, clientPort uint16, testPort uint16) {
	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", clientPort), socks.Version5, "", "")
	dialTCP := func() (net.Conn, error) {
		return dialer.DialContext(context.Background(), "tcp", M.ParseSocksaddrHostPort("10.0.0.1", testPort))
	}
	dialUDP := func() (net.PacketConn, error) {
		conn, err := dialer.DialContext(context.Background(), "udp", M.ParseSocksaddrHostPort("10.0.0.1", testPort))
		if err != nil {
			return nil, err
		}
		return bufio.NewUnbindPacketConn(conn), nil
	}
	require.NoError(t, testLargeDataWithConn(t, testPort, dialTCP))
	require.NoError(t, testLargeDataWithPacketConn(t, testPort, dialUDP))
	// require.NoError(t, testPingPongWithConn(t, testPort, dialTCP))
	// require.NoError(t, testPingPongWithPacketConn(t, testPort, dialUDP))
}
