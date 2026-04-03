package iprange

import (
	"context"
	"net"
	"sort"
	"sync"
)

func ResolveHostnames6(ctx context.Context, hosts []string, threads int) ([]Uint128, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(hosts) == 0 {
		return nil, nil
	}
	if threads <= 0 {
		threads = 1
	}

	type result struct {
		ips []Uint128
		err error
	}

	jobs := make(chan string)
	results := make(chan result, len(hosts))

	var wg sync.WaitGroup
	for range threads {
		wg.Go(func() {
			for host := range jobs {
				ips, err := resolve6(ctx, host)
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

	out := make([]Uint128, 0, len(hosts))
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
	sort.Slice(out, func(i, j int) bool {
		return out[i].LessThan(out[j])
	})
	return out, firstErr
}

func resolve6(ctx context.Context, hostname string) ([]Uint128, error) {
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, hostname)
	if err != nil {
		return nil, err
	}

	out := make([]Uint128, 0, len(addrs))
	for _, addr := range addrs {
		if ip4 := addr.IP.To4(); ip4 != nil {
			v4 := uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8 | uint32(ip4[3])
			out = append(out, IPv4ToMapped6(v4))
			continue
		}
		if len(addr.IP) == net.IPv6len {
			out = append(out, u128FromBytes(addr.IP))
		}
	}
	return out, nil
}
