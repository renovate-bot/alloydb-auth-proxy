// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy_test

import (
	"context"
	"errors"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cloud.google.com/go/alloydbconn"
	"github.com/GoogleCloudPlatform/alloydb-auth-proxy/internal/proxy"
	"github.com/spf13/cobra"
)

type fakeDialer struct{}

type testCase struct {
	desc          string
	in            *proxy.Config
	wantTCPAddrs  []string
	wantUnixAddrs []string
}

func (fakeDialer) Dial(ctx context.Context, inst string, opts ...alloydbconn.DialOption) (net.Conn, error) {
	conn, _ := net.Pipe()
	return conn, nil
}

func (fakeDialer) Close() error {
	return nil
}

type errorDialer struct {
	fakeDialer
}

func (errorDialer) Close() error {
	return errors.New("errorDialer returns error on Close")
}

func createTempDir(t *testing.T) (string, func()) {
	testDir, err := ioutil.TempDir("", "*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	return testDir, func() {
		if err := os.RemoveAll(testDir); err != nil {
			t.Logf("failed to cleanup temp dir: %v", err)
		}
	}
}

func TestClientInitialization(t *testing.T) {
	ctx := context.Background()
	testDir, cleanup := createTempDir(t)
	defer cleanup()
	inst1 := "/projects/proj/locations/region/clusters/clust/instances/inst1"
	inst2 := "/projects/proj/locations/region/clusters/clust/instances/inst2"
	wantUnix := "proj.region.clust.inst1"

	tcs := []testCase{
		{
			desc: "multiple instances",
			in: &proxy.Config{
				Addr: "127.0.0.1",
				Port: 5000,
				Instances: []proxy.InstanceConnConfig{
					{Name: inst1},
					{Name: inst2},
				},
			},
			wantTCPAddrs: []string{"127.0.0.1:5000", "127.0.0.1:5001"},
		},
		{
			desc: "with instance address",
			in: &proxy.Config{
				Addr: "1.1.1.1", // bad address, binding shouldn't happen here.
				Port: 5000,
				Instances: []proxy.InstanceConnConfig{
					{Addr: "0.0.0.0", Name: inst1},
				},
			},
			wantTCPAddrs: []string{"0.0.0.0:5000"},
		},
		{
			desc: "with instance port",
			in: &proxy.Config{
				Addr: "127.0.0.1",
				Port: 5000,
				Instances: []proxy.InstanceConnConfig{
					{Name: inst1, Port: 6000},
				},
			},
			wantTCPAddrs: []string{"127.0.0.1:6000"},
		},
		{
			desc: "with global port and instance port",
			in: &proxy.Config{
				Addr: "127.0.0.1",
				Port: 5000,
				Instances: []proxy.InstanceConnConfig{
					{Name: inst1},
					{Name: inst2, Port: 6000},
				},
			},
			wantTCPAddrs: []string{
				"127.0.0.1:5000",
				"127.0.0.1:6000",
			},
		},
		{
			desc: "with incrementing automatic port selection",
			in: &proxy.Config{
				Addr: "127.0.0.1",
				Port: 5432, // default port
				Instances: []proxy.InstanceConnConfig{
					{Name: inst1},
					{Name: inst2},
				},
			},
			wantTCPAddrs: []string{
				"127.0.0.1:5432",
				"127.0.0.1:5433",
			},
		},
		{
			desc: "with a Unix socket",
			in: &proxy.Config{
				UnixSocket: testDir,
				Instances: []proxy.InstanceConnConfig{
					{Name: inst1},
				},
			},
			wantUnixAddrs: []string{
				filepath.Join(testDir, wantUnix, ".s.PGSQL.5432"),
			},
		},
		{
			desc: "with a global TCP host port and an instance Unix socket",
			in: &proxy.Config{
				Addr: "127.0.0.1",
				Port: 5000,
				Instances: []proxy.InstanceConnConfig{
					{Name: inst1, UnixSocket: testDir},
				},
			},
			wantUnixAddrs: []string{
				filepath.Join(testDir, wantUnix, ".s.PGSQL.5432"),
			},
		},
		{
			desc: "with a global Unix socket and an instance TCP port",
			in: &proxy.Config{
				Addr:       "127.0.0.1",
				UnixSocket: testDir,
				Instances: []proxy.InstanceConnConfig{
					{Name: inst1, Port: 5000},
				},
			},
			wantTCPAddrs: []string{
				"127.0.0.1:5000",
			},
		},
	}
	_, isFlex := os.LookupEnv("FLEX")
	if !isFlex {
		tcs = append(tcs, testCase{
			desc: "IPv6 support",
			in: &proxy.Config{
				Addr: "::1",
				Port: 5000,
				Instances: []proxy.InstanceConnConfig{
					{Name: inst1},
				},
			},
			wantTCPAddrs: []string{"[::1]:5000"},
		})
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			tc.in.Dialer = fakeDialer{}
			c, err := proxy.NewClient(ctx, &cobra.Command{}, tc.in)
			if err != nil {
				t.Fatalf("want error = nil, got = %v", err)
			}
			defer c.Close()
			for _, addr := range tc.wantTCPAddrs {
				conn, err := net.Dial("tcp", addr)
				if err != nil {
					t.Fatalf("want error = nil, got = %v", err)
				}
				err = conn.Close()
				if err != nil {
					t.Logf("failed to close connection: %v", err)
				}
			}

			for _, addr := range tc.wantUnixAddrs {
				conn, err := net.Dial("unix", addr)
				if err != nil {
					t.Fatalf("want error = nil, got = %v", err)
				}
				err = conn.Close()
				if err != nil {
					t.Logf("failed to close connection: %v", err)
				}
			}
		})
	}
}

func TestClientClosesCleanly(t *testing.T) {
	in := &proxy.Config{
		Addr: "127.0.0.1",
		Port: 5000,
		Instances: []proxy.InstanceConnConfig{
			{Name: "proj:reg:inst"},
		},
		Dialer: fakeDialer{},
	}
	c, err := proxy.NewClient(context.Background(), &cobra.Command{}, in)
	if err != nil {
		t.Fatalf("proxy.NewClient error want = nil, got = %v", err)
	}
	go c.Serve(context.Background())
	time.Sleep(time.Second) // allow the socket to start listening

	conn, dErr := net.Dial("tcp", "127.0.0.1:5000")
	if dErr != nil {
		t.Fatalf("net.Dial error = %v", dErr)
	}
	_ = conn.Close()

	if err := c.Close(); err != nil {
		t.Fatalf("c.Close() error = %v", err)
	}
}

func TestClosesWithError(t *testing.T) {
	in := &proxy.Config{
		Addr: "127.0.0.1",
		Port: 5000,
		Instances: []proxy.InstanceConnConfig{
			{Name: "proj:reg:inst"},
		},
		Dialer: errorDialer{},
	}
	c, err := proxy.NewClient(context.Background(), &cobra.Command{}, in)
	if err != nil {
		t.Fatalf("proxy.NewClient error want = nil, got = %v", err)
	}
	go c.Serve(context.Background())
	time.Sleep(time.Second) // allow the socket to start listening

	if err = c.Close(); err == nil {
		t.Fatal("c.Close() should error, got nil")
	}
}

func TestMultiErrorFormatting(t *testing.T) {
	tcs := []struct {
		desc string
		in   proxy.MultiErr
		want string
	}{
		{
			desc: "with one error",
			in:   proxy.MultiErr{errors.New("woops")},
			want: "woops",
		},
		{
			desc: "with many errors",
			in:   proxy.MultiErr{errors.New("woops"), errors.New("another error")},
			want: "woops, another error",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			if got := tc.in.Error(); got != tc.want {
				t.Errorf("want = %v, got = %v", tc.want, got)
			}
		})
	}
}

func TestClientInitializationWorksRepeatedly(t *testing.T) {
	// The client creates a Unix socket on initial startup and does not remove
	// it on shutdown. This test ensures the existing socket does not cause
	// problems for a second invocation.
	ctx := context.Background()
	testDir, cleanup := createTempDir(t)
	defer cleanup()

	in := &proxy.Config{
		UnixSocket: testDir,
		Instances: []proxy.InstanceConnConfig{
			{Name: "/projects/proj/locations/region/clusters/clust/instances/inst1"},
		},
		Dialer: fakeDialer{},
	}
	c, err := proxy.NewClient(ctx, &cobra.Command{}, in)
	if err != nil {
		t.Fatalf("want error = nil, got = %v", err)
	}
	c.Close()

	c, err = proxy.NewClient(ctx, &cobra.Command{}, in)
	if err != nil {
		t.Fatalf("want error = nil, got = %v", err)
	}
	c.Close()
}
