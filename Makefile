build-and-run-oracle-v2-mainnet:
	go build -o ../bin/oracle-v2 ./cmd/oracle-v2 && install_name_tool -add_rpath /usr/local/lib ../bin/oracle-v2
	../bin/oracle-v2 --datadir ../datadirs/oracle/v2-mainnet-simple/ --sync-height-start 801000 --kernel-datadir /Volumes/T9/Bitcoin

build-and-run-oracle-v2-mainnet-libsecp:
	go build -tags=libsecp256k1 -o ../bin/oracle-v2-libsecp ./cmd/oracle-v2 && install_name_tool -add_rpath /usr/local/lib ../bin/oracle-v2-libsecp
	../bin/oracle-v2-libsecp --datadir ../datadirs/oracle/v2-mainnet-libsecp --sync-height-start 801000 --kernel-datadir /Volumes/T9/Bitcoin