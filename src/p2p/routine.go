package p2p

import (
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/peer"
	"github.com/btcsuite/btcd/wire"
	"net"
	"time"
)

func StartPeerRoutine(ph *PeerHandler, messageOutChan chan wire.Message, doneChan chan struct{}) {

	peerCfg := &peer.Config{
		UserAgentName:    "SilentPaymentAppGo", // User agent name to advertise.
		UserAgentVersion: "0.0.1",              // User agent version to advertise.
		//ChainParams:      &chaincfg.MainNetParams,
		ChainParams:     &chaincfg.RegressionNetParams,
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
		AllowSelfConns: true,
	}
	//p, err := peer.NewOutboundPeer(peerCfg, "192.168.178.25:8333")
	p, err := peer.NewOutboundPeer(peerCfg, "127.0.0.1:18444")
	if err != nil {
		fmt.Printf("NewOutboundPeer: error %v\n", err)
		return
	}

	// Establish the connection to the peer address and mark it connected.
	conn, err := net.Dial("tcp", p.Addr())
	if err != nil {
		fmt.Printf("net.Dial: error %v\n", err)
		return
	}
	p.AssociateConnection(conn)

	for {
		select {
		case <-doneChan:
			fmt.Println("was sent")
		case msg := <-messageOutChan:
			fmt.Println("message about to queue")
			p.QueueMessage(msg, doneChan)
			fmt.Println("message queued")
		case <-time.After(24 * time.Hour):
			fmt.Println("Ending program")
			p.Disconnect()
			p.WaitForDisconnect()
		}
	}
}
