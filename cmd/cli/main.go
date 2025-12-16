package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	pbAbf "github.com/FluVirus2/antibruteforce/api/gen/v1/antibruteforce"
	pbMgmt "github.com/FluVirus2/antibruteforce/api/gen/v1/antibruteforce_management"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	defaultServerAddr = "localhost:80"
	defaultTimeout    = 10 * time.Second
	serverAddrEnvKey  = "ABF_SERVER_ADDR"
)

var errInvalidUsage = errors.New("invalid usage")

func getDefaultServerAddr() string {
	if addr := os.Getenv(serverAddrEnvKey); addr != "" {
		return addr
	}
	return defaultServerAddr
}

func main() {
	if err := run(); err != nil {
		if !errors.Is(err, errInvalidUsage) {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		}
		os.Exit(1)
	}
}

func run() error {
	serverAddr := flag.String("server", getDefaultServerAddr(), "gRPC server address (env: ABF_SERVER_ADDR)")
	timeout := flag.Duration("timeout", defaultTimeout, "request timeout")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Anti-Bruteforce CLI\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <command> [arguments]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment:\n")
		fmt.Fprintf(os.Stderr, "  ABF_SERVER_ADDR    Server address (default: %s)\n", defaultServerAddr)
		fmt.Fprintf(os.Stderr, "\nCommands:\n")
		fmt.Fprintf(os.Stderr, "  ping                              Check server health\n")
		fmt.Fprintf(os.Stderr, "  check <login> <password> <ip>     Check access for credentials\n")
		fmt.Fprintf(os.Stderr, "  whitelist add <cidr>              Add subnet to whitelist\n")
		fmt.Fprintf(os.Stderr, "  whitelist remove <cidr>           Remove subnet from whitelist\n")
		fmt.Fprintf(os.Stderr, "  whitelist list                    List whitelist subnets\n")
		fmt.Fprintf(os.Stderr, "  blacklist add <cidr>              Add subnet to blacklist\n")
		fmt.Fprintf(os.Stderr, "  blacklist remove <cidr>           Remove subnet from blacklist\n")
		fmt.Fprintf(os.Stderr, "  blacklist list                    List blacklist subnets\n")
		fmt.Fprintf(os.Stderr, "  reset ip <ip>                     Reset rate limit bucket for IP\n")
		fmt.Fprintf(os.Stderr, "  reset login <login>               Reset rate limit bucket for login\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s ping\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s check admin password123 192.168.1.100\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s whitelist add 192.168.1.0/24\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -server localhost:8080 blacklist list\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s reset ip 192.168.1.100\n", os.Args[0])
	}

	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		return errInvalidUsage
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	conn, err := grpc.NewClient(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	abfClient := pbAbf.NewAntiBruteforceClient(conn)
	mgmtClient := pbMgmt.NewBruteforceManagementClient(conn)

	command := args[0]

	switch command {
	case "ping":
		return handlePing(ctx, abfClient)
	case "check":
		return handleCheck(ctx, abfClient, args[1:])
	case "whitelist":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: whitelist <add|remove|list> [args]")
			return errInvalidUsage
		}
		return handleWhitelist(ctx, mgmtClient, args[1], args[2:])
	case "blacklist":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: blacklist <add|remove|list> [args]")
			return errInvalidUsage
		}
		return handleBlacklist(ctx, mgmtClient, args[1], args[2:])
	case "reset":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: reset <ip|login> <value>")
			return errInvalidUsage
		}
		return handleReset(ctx, mgmtClient, args[1], args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", command)
		flag.Usage()
		return errInvalidUsage
	}
}

func handlePing(ctx context.Context, client pbAbf.AntiBruteforceClient) error {
	_, err := client.Ping(ctx, &emptypb.Empty{})
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	fmt.Println("pong")

	return nil
}

func handleCheck(ctx context.Context, client pbAbf.AntiBruteforceClient, args []string) error {
	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: check <login> <password> <ip>")
		return errInvalidUsage
	}

	login := args[0]
	password := args[1]
	ip := args[2]

	resp, err := client.CheckAccess(ctx, &pbAbf.CheckAccessRequest{
		Login:    login,
		Password: password,
		Ip:       ip,
	})
	if err != nil {
		return fmt.Errorf("check access failed: %w", err)
	}

	if resp.Allowed {
		fmt.Println("Access: ALLOWED")
	} else {
		fmt.Printf("Access: DENIED (%s)\n", formatDeniedReason(resp.Reason))
	}

	return nil
}

func formatDeniedReason(reason pbAbf.AccessDeniedReason) string {
	switch reason {
	case pbAbf.AccessDeniedReason_ACCESS_DENIED_REASON_IP_BLACK_LIST:
		return "IP is blacklisted"
	case pbAbf.AccessDeniedReason_ACCESS_DENIED_REASON_LOGIN_BLACK_LIST:
		return "login is blacklisted"
	case pbAbf.AccessDeniedReason_ACCESS_DENIED_REASON_TOO_MANY_REQUESTS_IP:
		return "too many requests from IP"
	case pbAbf.AccessDeniedReason_ACCESS_DENIED_REASON_TOO_MANY_REQUESTS_LOGIN:
		return "too many requests for login"
	case pbAbf.AccessDeniedReason_ACCESS_DENIED_REASON_TOO_MANY_REQUESTS_PASSWORD:
		return "too many requests for password"
	case pbAbf.AccessDeniedReason_ACCESS_DENIED_REASON_UNSPECIFIED:
		return "unspecified reason"
	default:
		return "unknown reason"
	}
}

//nolint:dupl,lll
func handleWhitelist(ctx context.Context, client pbMgmt.BruteforceManagementClient, subcommand string, args []string) error {
	switch subcommand {
	case "add":
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "usage: whitelist add <cidr>")
			return errInvalidUsage
		}

		_, err := client.AddIPToWhiteList(ctx, &pbMgmt.SubnetRequest{
			Subnet: &pbMgmt.Subnet{Cidr: args[0]},
		})
		if err != nil {
			return fmt.Errorf("failed to add to whitelist: %w", err)
		}

		fmt.Printf("Added %s to whitelist\n", args[0])

	case "remove":
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "usage: whitelist remove <cidr>")
			return errInvalidUsage
		}

		_, err := client.RemoveIPFromWhiteList(ctx, &pbMgmt.SubnetRequest{
			Subnet: &pbMgmt.Subnet{Cidr: args[0]},
		})
		if err != nil {
			return fmt.Errorf("failed to remove from whitelist: %w", err)
		}

		fmt.Printf("Removed %s from whitelist\n", args[0])

	case "list":
		resp, err := client.ListIPAddressWhiteList(ctx, &pbMgmt.ListSubnetsRequest{
			Pagination: &pbMgmt.Pagination{Offset: 0, Limit: 1000},
		})
		if err != nil {
			return fmt.Errorf("failed to list whitelist: %w", err)
		}

		if len(resp.Subnets) == 0 {
			fmt.Println("Whitelist is empty")
			return nil
		}

		fmt.Println("Whitelist:")
		for _, subnet := range resp.Subnets {
			fmt.Printf("  %s\n", subnet)
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown whitelist subcommand: %s\n", subcommand)
		return errInvalidUsage
	}

	return nil
}

//nolint:dupl,lll
func handleBlacklist(ctx context.Context, client pbMgmt.BruteforceManagementClient, subcommand string, args []string) error {
	switch subcommand {
	case "add":
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "usage: blacklist add <cidr>")
			return errInvalidUsage
		}

		_, err := client.AddIPToBlackList(ctx, &pbMgmt.SubnetRequest{
			Subnet: &pbMgmt.Subnet{Cidr: args[0]},
		})
		if err != nil {
			return fmt.Errorf("failed to add to blacklist: %w", err)
		}

		fmt.Printf("Added %s to blacklist\n", args[0])

	case "remove":
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "usage: blacklist remove <cidr>")
			return errInvalidUsage
		}

		_, err := client.RemoveIPFromBlackList(ctx, &pbMgmt.SubnetRequest{
			Subnet: &pbMgmt.Subnet{Cidr: args[0]},
		})
		if err != nil {
			return fmt.Errorf("failed to remove from blacklist: %w", err)
		}

		fmt.Printf("Removed %s from blacklist\n", args[0])

	case "list":
		resp, err := client.ListIPAddressBlackList(ctx, &pbMgmt.ListSubnetsRequest{
			Pagination: &pbMgmt.Pagination{Offset: 0, Limit: 1000},
		})
		if err != nil {
			return fmt.Errorf("failed to list blacklist: %w", err)
		}

		if len(resp.Subnets) == 0 {
			fmt.Println("Blacklist is empty")
			return nil
		}

		fmt.Println("Blacklist:")
		for _, subnet := range resp.Subnets {
			fmt.Printf("  %s\n", subnet)
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown blacklist subcommand: %s\n", subcommand)
		return errInvalidUsage
	}

	return nil
}

//nolint:lll
func handleReset(ctx context.Context, client pbMgmt.BruteforceManagementClient, subcommand string, args []string) error {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: reset %s <value>\n", subcommand)
		return errInvalidUsage
	}

	switch subcommand {
	case "ip":
		resp, err := client.ResetBucketByIP(ctx, &pbMgmt.ResetBucketByIPRequest{Ip: args[0]})
		if err != nil {
			return fmt.Errorf("failed to reset IP bucket: %w", err)
		}

		if resp.WasDone {
			fmt.Printf("Reset bucket for IP %s\n", args[0])
		} else {
			fmt.Printf("No bucket found for IP %s\n", args[0])
		}

	case "login":
		resp, err := client.ResetBucketByLogin(ctx, &pbMgmt.ResetBucketByLoginRequest{Login: args[0]})
		if err != nil {
			return fmt.Errorf("failed to reset login bucket: %w", err)
		}

		if resp.WasDone {
			fmt.Printf("Reset bucket for login %s\n", args[0])
		} else {
			fmt.Printf("No bucket found for login %s\n", args[0])
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown reset subcommand: %s\n", subcommand)
		fmt.Fprintln(os.Stderr, "available: ip, login")

		return errInvalidUsage
	}

	return nil
}
