package main

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"
	"github.com/rs/cors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/dangthanhduong01/simplebank/api"
	db "github.com/dangthanhduong01/simplebank/db/sqlc"
	"github.com/dangthanhduong01/simplebank/db/utils"
	"github.com/dangthanhduong01/simplebank/gapi"
	"github.com/dangthanhduong01/simplebank/mail"
	"github.com/dangthanhduong01/simplebank/pb"
	"github.com/dangthanhduong01/simplebank/worker"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	// "github.com/rakyll/statik/fs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"

	_ "github.com/lib/pq"
)

var interruptSignals = []os.Signal{
	os.Interrupt,
	syscall.SIGTERM,
	syscall.SIGINT,
}

func main() {
	config, err := utils.LoadConfig(".")
	if err != nil {
		log.Fatal().Msg("Cannot load config:")
	}

	if config.Environment == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	ctx, stop := signal.NotifyContext(context.Background(), interruptSignals...)
	defer stop()

	conn, err := sql.Open(config.DBDriver, config.DBSource)
	if err != nil {
		log.Fatal().Msg("cannot connect to db:")
	}

	store := db.NewStore(conn)

	redisOpt := asynq.RedisClientOpt{
		Addr: config.RedisAddress,
	}

	taskDistributor := worker.NewRedisTaskDistributor(redisOpt)

	waitGroup, ctx := errgroup.WithContext(ctx)

	runTaskProcessor(ctx, waitGroup, config, redisOpt, store)
	go runGatewayServer(ctx, waitGroup, config, store, taskDistributor)
	runGrpcServer(ctx, waitGroup, config, store, taskDistributor)

	err = waitGroup.Wait()
	if err != nil {
		log.Fatal().Err(err).Msg("error group error")
	}

	// server, err := api.NewServer(config, store)
	// if err != nil {
	// 	log.Fatal().Msg("cannot create server:", err)
	// }
	// err = server.Start(config.HTTPServerAddress)
	// if err != nil {
	// 	log.Fatal().Msg("cannot start server:", err)
	// }
}

// func runDBMigration(migrationURL string, dbSource string) {
// 	migration, err := migrate.New(migrationURL, dbSource)
// 	if err != nil {
// 		log.Fatal().Msg("cannot create new migrate instance")
// 	}

// 	if err = migration.Up(); err != nil && err != migrate.ErrNoChange {
// 		log.Fatal().Msg("failed to migrate up")
// 	}

// 	log.Info().Msg("cannot create server")
// }

func runTaskProcessor(ctx context.Context, waitGroup *errgroup.Group,
	config utils.Config, redisOpt asynq.RedisClientOpt, store db.Store) {
	mailer := mail.NewGmailSender(config.EmailSenderName, config.EmailSenderAddress, config.EmailSenderPassword)
	taskProcessor := worker.NewRedisTaskProcessor(redisOpt, store, mailer)
	log.Info().Msg("start task processor")
	err := taskProcessor.Start()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to start task processor")
	}

	waitGroup.Go(func() error {
		<-ctx.Done()
		log.Info().Msg("greatful shutting down task processor")
		taskProcessor.Shutdown()
		log.Info().Msg("task processor shutdown")
		return nil
	})
}

func runGrpcServer(ctx context.Context, waitGroup *errgroup.Group,
	config utils.Config, store db.Store, taskDistributor worker.TaskDistributor) {
	server, err := gapi.NewServer(config, store, taskDistributor)
	if err != nil {
		log.Fatal().Msg("cannot create server:")
	}

	grpcLogger := grpc.UnaryInterceptor(gapi.GrpcLogger)
	grpcServer := grpc.NewServer(grpcLogger)
	pb.RegisterSimpleBankServer(grpcServer, server)
	reflection.Register(grpcServer)

	listener, err := net.Listen("tcp", config.GRPCServerAddress)
	if err != nil {
		log.Fatal().Msg("cannot create listener:")
	}

	waitGroup.Go(func() error {
		log.Info().Msgf("start gRPC server at %s", listener.Addr().String())

		err = grpcServer.Serve(listener)
		if err != nil {
			if errors.Is(err, grpc.ErrServerStopped) {
				return nil
			}
			log.Error().Msg("cannot start gRPC server:")
			return nil
		}

		return nil
	})

	waitGroup.Go(func() error {
		<-ctx.Done()
		log.Info().Msg("shutting down gRPC server")
		grpcServer.GracefulStop()
		return nil
	})

	// log.Info().Msgf("start gRPC server at %s", listener.Addr().String())
	// err = grpcServer.Serve(listener)
	// if err != nil {
	// 	log.Fatal().Msg("cannot start gRPC server:")
	// }
}

func runGatewayServer(ctx context.Context, waitGroup *errgroup.Group,
	config utils.Config, store db.Store, taskDistributor worker.TaskDistributor) {
	server, err := gapi.NewServer(config, store, taskDistributor)
	if err != nil {
		log.Fatal().Msg("cannot create server:")
	}

	jsonOption := runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
		MarshalOptions: protojson.MarshalOptions{
			UseProtoNames: true,
		},
		UnmarshalOptions: protojson.UnmarshalOptions{
			DiscardUnknown: true,
		},
	})

	grpcMux := runtime.NewServeMux(jsonOption)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = pb.RegisterSimpleBankHandlerServer(ctx, grpcMux, server)
	if err != nil {
		log.Fatal().Msg("cannot register handler server")
	}

	mux := http.NewServeMux()
	mux.Handle("/", grpcMux)

	// statikFS, err := fs.New()
	// if err != nil {
	// 	log.Fatal("cannot create statik fs")
	// }
	// swaggerHandler := http.StripPrefix("/swagger", http.FileServer(statikFS))
	// mux.Handle("/swagger/", swaggerHandler)

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
		AllowedHeaders:   []string{"Authorization"},
	})

	handler := c.Handler(gapi.HttpLogger(mux))
	httpServer := &http.Server{
		Addr:    config.HTTPServerAddress,
		Handler: handler,
	}

	waitGroup.Go(func() error {
		log.Info().Msgf("start HTTP gateway server at %s", httpServer.Addr)
		err := httpServer.ListenAndServe()
		// err = http.Serve(listener, handler)
		if err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}

			log.Fatal().Msg("cannot start http Gateway server: ")
			return err
		}
		return nil
	})

	waitGroup.Go(func() error {
		<-ctx.Done()
		log.Info().Msg("shutting down HTTP gateway server")

		err := httpServer.Shutdown(context.Background())
		if err != nil {
			log.Error().Msg("cannot shutdown http gateway server")
		}

		log.Info().Msg("http gateway server shutdown")
		return nil
	})

	// listener, err := net.Listen("tcp", config.HTTPServerAddress)
	// if err != nil {
	// 	log.Fatal().Msg("cannot create listener:")
	// }

	// log.Info().Msgf("start HTTP gateway server at %s", listener.Addr().String())
	// handler := gapi.HttpLogger(mux)
	// err = http.Serve(listener, handler)
	// if err != nil {
	// 	log.Fatal().Msg("cannot start http Gateway server: ")
	// }
}

func runGinServer(config utils.Config, store db.Store) {
	server, err := api.NewServer(config, store)
	if err != nil {
		log.Fatal().Msg("cannot create server:")
	}

	err = server.Start(config.HTTPServerAddress)
	if err != nil {
		log.Fatal().Msg("cannot start server:")
	}
}
