package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v4"
	db "github.com/niiniyare/awo/db/sqlc"

	// _ "github.com/niiniyare/awo/doc/statik"
	"github.com/niiniyare/awo/util"
	// "github.com/rs/zerolog"
	// "github.com/rs/zerolog/log"
)

func main() {
	config, err := util.LoadConfig(".")
	if err != nil {
		log.Fatal(err)
	}
	//TODO: setup Logger
	// if config.Environment == "development" {
	// 	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	// }
	ctx := context.Background()

	conn, err := pgx.Connect(ctx, config.DBSource)
	if err != nil {
		log.Fatalln("testDB failed to connect:", err)
	}

	defer conn.Close(ctx)
	if err != nil {
		log.Fatal("cannot connect to db:", err)
	}

	store := db.NewStore(conn)
	fmt.Println(store)

	// go runGatewayServer(config, store)
	// runGrpcServer(config, storev)
}

//
// func runGrpcServer(config util.Config, store db.Store) {
// 	server, err := gapi.NewServer(config, store, taskDistributor)
// 	if err != nil {
// 		log.Fatal().Err(err).Msg("cannot create server")
// 	}
//
// 	gprcLogger := grpc.UnaryInterceptor(gapi.GrpcLogger)
// 	grpcServer := grpc.NewServer(gprcLogger)
// 	pb.RegisterSimpleBankServer(grpcServer, server)
// 	reflection.Register(grpcServer)
//
// 	listener, err := net.Listen("tcp", config.GRPCServerAddress)
// 	if err != nil {
// 		log.Fatal().Err(err).Msg("cannot create listener")
// 	}
//
// 	log.Info().Msgf("start gRPC server at %s", listener.Addr().String())
// 	err = grpcServer.Serve(listener)
// 	if err != nil {
// 		log.Fatal().Err(err).Msg("cannot start gRPC server")
// 	}
// }
//
// func runGatewayServer(config util.Config, store db.Store) {
// 	server, err := NewServer(config, store, taskDistributor)
// 	if err != nil {
// 		log.Fatal().Err(err).Msg("cannot create server")
// 	}
//
// 	jsonOption := runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
// 		MarshalOptions: protojson.MarshalOptions{
// 			UseProtoNames: true,
// 		},
// 		UnmarshalOptions: protojson.UnmarshalOptions{
// 			DiscardUnknown: true,
// 		},
// 	})
//
// 	grpcMux := runtime.NewServeMux(jsonOption)
//
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()
//
// 	err = pb.RegisterSimpleBankHandlerServer(ctx, grpcMux, server)
// 	if err != nil {
// 		log.Fatal().Err(err).Msg("cannot register handler server")
// 	}
//
// 	mux := http.NewServeMux()
// 	mux.Handle("/", grpcMux)
//
// 	statikFS, err := fs.New()
// 	if err != nil {
// 		log.Fatal().Err(err).Msg("cannot create statik fs")
// 	}
//
// 	swaggerHandler := http.StripPrefix("/swagger/", http.FileServer(statikFS))
// 	mux.Handle("/swagger/", swaggerHandler)
//
// 	listener, err := net.Listen("tcp", config.HTTPServerAddress)
// 	if err != nil {
// 		log.Fatal().Err(err).Msg("cannot create listener")
// 	}
//
// 	log.Info().Msgf("start HTTP gateway server at %s", listener.Addr().String())
// 	handler := gapi.HttpLogger(mux)
// 	err = http.Serve(listener, handler)
// 	if err != nil {
// 		log.Fatal().Err(err).Msg("cannot start HTTP gateway server")
// 	}
// }
