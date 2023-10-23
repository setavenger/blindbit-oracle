package p2p

import (
	"SilentPaymentAppBackend/src/common"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/peer"
	"github.com/btcsuite/btcd/wire"
	"log"
	"net"
	"time"
)

// todo implement reconnect

func StartPeerRoutine(ph *PeerHandler, messageOutChan chan wire.Message, doneChan chan struct{}, endedChan chan struct{}) {

	disconnectedChan := make(chan struct{})
	peerCfg := &peer.Config{
		UserAgentName:    "SilentPaymentAppGo", // User agent name to advertise.
		UserAgentVersion: "0.0.1",              // User agent version to advertise.
		//ChainParams:      &chaincfg.MainNetParams,
		ChainParams:     &chaincfg.SigNetParams,
		Services:        0,
		ProtocolVersion: 70016,
		TrickleInterval: time.Second * 10,
		Listeners: peer.MessageListeners{
			OnVersion: ph.onVersion,
			OnVerAck:  ph.onVerack,
			OnInv:     ph.onInv,
			OnCFilter: ph.onCFilter,
			OnTx:      ph.onTx,
			OnPong:    ph.onPong,
			OnBlock:   ph.onBlock,
			OnHeaders: ph.onHeaders,
		},
	}
	/*
		Signet
		178.128.221.177:38333
		103.16.128.63:38333
		153.126.143.201:38333
		192.241.163.142:38333
	*/
	//p, err := peer.NewOutboundPeer(peerCfg, "192.168.178.25:8333")  // umbrel mainnet
	//p, err := peer.NewOutboundPeer(peerCfg, "127.0.0.1:18444")  // regtest
	p, err := peer.NewOutboundPeer(peerCfg, "153.126.143.201:38333")
	if err != nil {
		log.Printf("NewOutboundPeer: error %v\n", err)
		return
	}

	// Establish the connection to the peer address and mark it connected.
	conn, err := net.Dial("tcp", p.Addr())
	if err != nil {
		log.Printf("net.Dial: error %v\n", err)
		return
	}
	p.AssociateConnection(conn)

	go func() {
		for true {
			<-time.After(1 * time.Minute)
			if !p.Connected() {
				log.Println("Disconnected from peer")
				disconnectedChan <- struct{}{}
			}
		}
	}()

	for {
		select {
		case <-doneChan:
			common.DebugLogger.Println("message was sent out")
		case msg := <-messageOutChan:
			common.DebugLogger.Println("message about to queue")
			p.QueueMessage(msg, doneChan)
			common.DebugLogger.Println("message queued")
		case <-disconnectedChan:
			endedChan <- struct{}{}
			return
			//case <-time.After(24 * time.Hour):
			//	log.Println("Ending program")
			//	p.Disconnect()
			//	p.WaitForDisconnect()
		}
	}
}
