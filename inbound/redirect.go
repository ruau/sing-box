package inbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/redir"
	"github.com/sagernet/sing-box/common/script"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type Redirect struct {
	myInboundAdapter

	// Script
	scripts []*script.Script
}

func NewRedirect(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.RedirectInboundOptions) (*Redirect, error) {
	redirect := &Redirect{
		myInboundAdapter: myInboundAdapter{
			protocol:      C.TypeRedirect,
			network:       []string{N.NetworkTCP},
			ctx:           ctx,
			router:        router,
			logger:        logger,
			tag:           tag,
			listenOptions: options.ListenOptions,
		},
	}
	redirect.connHandler = redirect

	// Script
	if len(options.Scripts) > 0 {
		redirect.scripts = make([]*script.Script, 0, len(options.Scripts))
		for _, opt := range options.Scripts {
			s, err := script.NewScript(ctx, logger, opt)
			if err != nil {
				return nil, E.Cause(err, "initialize script")
			}
			redirect.scripts = append(redirect.scripts, s)
		}
	}

	return redirect, nil
}

func (r *Redirect) Start() error {
	if len(r.scripts) > 0 {
		// Script
		for i, s := range r.scripts {
			err := s.CallWithEvent(r.ctx, script.EventBeforeStart)
			if err != nil {
				return E.Cause(err, "call script[", i, "] ", script.EventBeforeStart)
			}
		}
	}
	err := r.myInboundAdapter.Start()
	if err != nil {
		if len(r.scripts) > 0 {
			// Script
			for _, s := range r.scripts {
				s.CallWithEvent(r.ctx, script.EventStartFailed)
			}
		}
		return err
	}
	defer func() {
		if err != nil && len(r.scripts) > 0 {
			// Script
			for _, s := range r.scripts {
				s.CallWithEvent(r.ctx, script.EventStartFailed)
			}
		}
	}()
	if len(r.scripts) > 0 {
		// Script
		for i, s := range r.scripts {
			err = s.CallWithEvent(r.ctx, script.EventAfterStart)
			if err != nil {
				return E.Cause(err, "call script[", i, "] ", script.EventAfterStart)
			}
		}
	}
	return nil
}

func (r *Redirect) Close() error {
	if len(r.scripts) > 0 {
		// Script
		for _, s := range r.scripts {
			s.CallWithEvent(context.Background(), script.EventBeforeClose)
		}
	}
	err := r.myInboundAdapter.Close()
	if len(r.scripts) > 0 {
		// Script
		for _, s := range r.scripts {
			s.CallWithEvent(context.Background(), script.EventAfterClose)
			s.Close()
		}
	}
	return err
}

func (r *Redirect) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	destination, err := redir.GetOriginalDestination(conn)
	if err != nil {
		return E.Cause(err, "get redirect destination")
	}
	metadata.Destination = M.SocksaddrFromNetIP(destination)
	return r.newConnection(ctx, conn, metadata)
}
