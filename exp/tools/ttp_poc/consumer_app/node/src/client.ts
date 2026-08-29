import * as grpc from '@grpc/grpc-js';
import { token_transfer_service } from '../gen/event_service/event_service';
import { token_transfer } from '../gen/ingest/processors/token_transfer/token_transfer_event';

// Create a gRPC client
export class EventServiceClient {
    private client: token_transfer_service.EventServiceClient;

    constructor(serverAddress: string = 'localhost:50051') {
        this.client = new token_transfer_service.EventServiceClient(
            serverAddress,
            grpc.credentials.createInsecure()
        );
    }

    // Get events stream
    getTTPEvents(
        startLedger: number, 
        endLedger: number, 
        onEvent: (event: token_transfer.TokenTransferEvent) => void
    ): void {
        const request = new token_transfer_service.GetEventsRequest({
            start_ledger: startLedger,
            end_ledger: endLedger
        });
        const stream = this.client.GetTTPEvents(request);

        stream.on('data', (event) => {
            onEvent(event);
        });

        stream.on('error', (error) => {
            console.error('Error in getTTPEvents stream:', error);
            throw error;
        });

        // Block until stream ends
        stream.on('end', () => {
            return;
        });
    }
} 