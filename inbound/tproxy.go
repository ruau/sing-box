package inbound

import (
	"context"
	"net"
	"net/netip"
	"syscall"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/redir"
	"github.com/sagernet/sing-box/common/script"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/udpnat"
)

type TProxy struct {
	myInboundAdapter
	udpNat *udpnat.Service[netip.AddrPort]

	// Script
	scripts []*script.Script
}

func NewTProxy(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.TProxyInboundOptions) (*TProxy, error) {
	tproxy := &TProxy{
		myInboundAdapter: myInboundAdapter{
			protocol:      C.TypeTProxy,
			network:       options.Network.Build(),
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
	}
	var udpTimeout time.Duration
	if options.UDPTimeout != 0 {
		udpTimeout = time.Duration(options.UDPTimeout)
	} else {
		udpTimeout = C.UDPTimeout
	}
	tproxy.connHandler = tproxy
	tproxy.oobPacketHandler = tproxy
	tproxy.udpNat = udpnat.New[netip.AddrPort](int64(udpTimeout.Seconds()), tproxy.upstreamContextHandler())
	tproxy.packetUpstream = tproxy.udpNat

	// Script
	if len(options.Scripts) > 0 {
		tproxy.scripts = make([]*script.Script, 0, len(options.Scripts))
		for _, opt := range options.Scripts {
			s, err := script.NewScript(ctx, logger, opt)
			if err != nil {
				return nil, E.Cause(err, "initialize script")
			}
			tproxy.scripts = append(tproxy.scripts, s)
		}
	}

	return tproxy, nil
}

func (t *TProxy) Start() error {
	if len(t.scripts) > 0 {
		// Script
		for i, s := range t.scripts {
			err := s.CallWithEvent(t.ctx, script.EventBeforeStart)
			if err != nil {
				return E.Cause(err, "call script[", i, "] ", script.EventBeforeStart)
			}
		}
	}
	err := t.myInboundAdapter.Start()
	if err != nil {
		if len(t.scripts) > 0 {
			// Script
			for _, s := range t.scripts {
				s.CallWithEvent(t.ctx, script.EventStartFailed)
			}
		}
		return err
	}
	defer func() {
		if err != nil && len(t.scripts) > 0 {
			// Script
			for _, s := range t.scripts {
				s.CallWithEvent(t.ctx, script.EventStartFailed)
			}
		}
	}()
	if len(t.scripts) > 0 {
		// Script
		for i, s := range t.scripts {
			err = s.CallWithEvent(t.ctx, script.EventAfterStart)
			if err != nil {
				return E.Cause(err, "call script[", i, "] ", script.EventAfterStart)
			}
		}
	}
	if t.tcpListener != nil {
		err = control.Conn(common.MustCast[syscall.Conn](t.tcpListener), func(fd uintptr) error {
			return redir.TProxy(fd, M.SocksaddrFromNet(t.tcpListener.Addr()).Addr.Is6())
		})
		if err != nil {
			return E.Cause(err, "configure tproxy TCP listener")
		}
	}
	if t.udpConn != nil {
		err = control.Conn(t.udpConn, func(fd uintptr) error {
			return redir.TProxy(fd, M.SocksaddrFromNet(t.udpConn.LocalAddr()).Addr.Is6())
		})
		if err != nil {
			return E.Cause(err, "configure tproxy UDP listener")
		}
	}
	return nil
}

func (t *TProxy) Close() error {
	if len(t.scripts) > 0 {
		// Script
		for _, s := range t.scripts {
			s.CallWithEvent(context.Background(), script.EventBeforeClose)
		}
	}
	err := t.myInboundAdapter.Close()
	if len(t.scripts) > 0 {
		// Script
		for _, s := range t.scripts {
			s.CallWithEvent(context.Background(), script.EventAfterClose)
		}
	}
	return err
}

func (t *TProxy) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	metadata.Destination = M.SocksaddrFromNet(conn.LocalAddr()).Unwrap()
	return t.newConnection(ctx, conn, metadata)
}

func (t *TProxy) NewPacket(ctx context.Context, conn N.PacketConn, buffer *buf.Buffer, oob []byte, metadata adapter.InboundContext) error {
	destination, err := redir.GetOriginalDestinationFromOOB(oob)
	if err != nil {
		return E.Cause(err, "get tproxy destination")
	}
	metadata.Destination = M.SocksaddrFromNetIP(destination).Unwrap()
	t.udpNat.NewContextPacket(ctx, metadata.Source.AddrPort(), buffer, adapter.UpstreamMetadata(metadata), func(natConn N.PacketConn) (context.Context, N.PacketWriter) {
		return adapter.WithContext(log.ContextWithNewID(ctx), &metadata), &tproxyPacketWriter{ctx: ctx, source: natConn, destination: metadata.Destination}
	})
	return nil
}

type tproxyPacketWriter struct {
	ctx         context.Context
	source      N.PacketConn
	destination M.Socksaddr
	conn        *net.UDPConn
}

func (w *tproxyPacketWriter) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	defer buffer.Release()
	conn := w.conn
	if w.destination == destination && conn != nil {
		_, err := conn.WriteToUDPAddrPort(buffer.Bytes(), M.AddrPortFromNet(w.source.LocalAddr()))
		if err != nil {
			w.conn = nil
		}
		return err
	}
	var listener net.ListenConfig
	listener.Control = control.Append(listener.Control, control.ReuseAddr())
	listener.Control = control.Append(listener.Control, redir.TProxyWriteBack())
	packetConn, err := listener.ListenPacket(w.ctx, "udp", destination.String())
	if err != nil {
		return err
	}
	udpConn := packetConn.(*net.UDPConn)
	if w.destination == destination {
		w.conn = udpConn
	} else {
		defer udpConn.Close()
	}
	return common.Error(udpConn.WriteToUDPAddrPort(buffer.Bytes(), M.AddrPortFromNet(w.source.LocalAddr())))
}

func (w *tproxyPacketWriter) Close() error {
	return common.Close(common.PtrOrNil(w.conn))
}
