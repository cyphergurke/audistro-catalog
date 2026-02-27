package proofcheck

import (
	"context"
	"net"
)

type NetDNSResolver struct {
	resolver *net.Resolver
}

func NewNetDNSResolver() *NetDNSResolver {
	return &NetDNSResolver{resolver: net.DefaultResolver}
}

func (r *NetDNSResolver) LookupTXT(ctx context.Context, domain string) ([]string, error) {
	return r.resolver.LookupTXT(ctx, domain)
}
