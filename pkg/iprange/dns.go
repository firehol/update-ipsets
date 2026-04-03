package iprange

import (
	"context"
	"net"
	"sort"
	"sync"
)

type Resolver interface {
	LookupIPv4(ctx context.Context, host string) ([]uint32, error)
}

type DefaultResolver struct{}

func (DefaultResolver) LookupIPv4(ctx context.Context, host string) ([]uint32, error) {
	addrs, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
	if err != nil {
		return nil, err
	}
	out := make([]uint32, 0, len(addrs))
	for _, addr := range addrs {
		ip4 := addr.To4()
		if ip4 == nil {
			continue
		}
		out = append(out, uint32(ip4[0])<<24|uint32(ip4[1])<<16|uint32(ip4[2])<<8|uint32(ip4[3]))
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

func ResolveHostnames(ctx context.Context, hosts []string, threads int, resolver Resolver) ([]uint32, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(hosts) == 0 {
		return nil, nil
	}
	if threads <= 0 {
		threads = 1
	}
	if resolver == nil {
		resolver = DefaultResolver{}
	}

	type result struct {
		ips []uint32
		err error
	}

	jobs := make(chan string)
	results := make(chan result, len(hosts))

	var wg sync.WaitGroup
	for range threads {
		wg.Go(func() {
			for host := range jobs {
				ips, err := resolver.LookupIPv4(ctx, host)
				results <- result{ips: ips, err: err}
			}
		})
	}

	go func() {
		for _, host := range hosts {
			select {
			case <-ctx.Done():
				close(jobs)
				wg.Wait()
				close(results)
				return
			case jobs <- host:
			}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	out := make([]uint32, 0, len(hosts))
	var firstErr error
	for res := range results {
		if res.err != nil {
			if firstErr == nil {
				firstErr = res.err
			}
			continue
		}
		out = append(out, res.ips...)
	}
	if err := ctx.Err(); err != nil && firstErr == nil {
		firstErr = err
	}
	return out, firstErr
}
