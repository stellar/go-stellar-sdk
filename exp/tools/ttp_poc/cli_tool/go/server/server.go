package server

import (
	"cli_tool/gen/event_service"
	"log"
	"time"

	"github.com/stellar/go/ingest/processors/token_transfer"
	"github.com/stellar/go/xdr"
	"github.com/stellar/stellar-rpc/client"
	"github.com/stellar/stellar-rpc/protocol"
)

type EventServer struct {
	event_service.UnimplementedEventServiceServer
	rpcClient *client.Client
	processor *token_transfer.EventsProcessor
}

func NewEventServer(rpcEndpoint string, passphrase string) (*EventServer, error) {
	rpcClient := client.NewClient(rpcEndpoint, nil)
	processor := token_transfer.NewEventsProcessor(passphrase)
	return &EventServer{
		rpcClient: rpcClient,
		processor: processor,
	}, nil
}

func (s *EventServer) GetTTPEvents(req *event_service.GetEventsRequest, stream event_service.EventService_GetTTPEventsServer) error {
	ctx := stream.Context()

	// Create initial RPC GetLedgers request
	getLedgersReq := protocol.GetLedgersRequest{
		StartLedger: uint32(req.StartLedger),
		Pagination:  &protocol.LedgerPaginationOptions{Limit: 1},
	}

	// this example code acquires ledger metadata from an RPC server instance
	// other sources of ledger metadata could be used as alternatives
	// such as ledgerbackend's from the Ingest SDK for GCS buckets or Captive core.

	// this routine does a naive, inefficient long polling approach against the RPC
	// with every RPC request getting a response with page size of just one ledger
	for {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			err := ctx.Err()
			log.Printf("Context cancelled in GetTTPEvents: %v", err)
			return err
		default:
		}

		// Get ledgers from RPC
		resp, err := s.rpcClient.GetLedgers(ctx, getLedgersReq)
		if err != nil {
			log.Printf("Error getting ledgers from RPC: %+v", err)
			return err
		}

		if len(resp.Ledgers) == 0 {
			// No ledgers, wait a bit before polling again
			time.Sleep(5 * time.Second)
			continue
		}

		lastSeq := uint32(0)
		// Process each ledger
		for _, ledgerInfo := range resp.Ledgers {
			// Convert LedgerMetadata in base64 encoding from RPC to xdr.LedgerCloseMeta
			var ledgerMeta xdr.LedgerCloseMeta
			if err := xdr.SafeUnmarshalBase64(ledgerInfo.LedgerMetadata, &ledgerMeta); err != nil {
				log.Printf("Error unmarshaling ledger metadata: %v: %v", ledgerInfo.LedgerMetadata, err)
				return err
			}
			lastSeq = ledgerMeta.LedgerSequence()

			// Get events from ledger metadata using the token_transfer processor
			events, err := s.processor.EventsFromLedger(ledgerMeta)
			if err != nil {
				log.Printf("Error processing events from ledger %d: %v", ledgerInfo.Sequence, err)
				return err
			}

			// Stream each token transfer event
			for i := range events {
				ttpEvent := events[i]
				if err := stream.Send(ttpEvent); err != nil {
					log.Printf("Error sending event to stream: %v", err)
					return err
				}
			}
		}

		// if this was bounded range request, we're done now.
		if req.EndLedger > 0 && lastSeq >= req.EndLedger {
			return nil
		}

		// naive approach for unbounded ledger retrieval with long polling
		// pause to stay at least one ledger back from latest ledger on rpc
		if lastSeq >= resp.LatestLedger-1 {
			time.Sleep(7 * time.Second)
		}

		// Update cursor for next request
		getLedgersReq.Pagination.Cursor = resp.Cursor
		getLedgersReq.StartLedger = 0
	}
}
