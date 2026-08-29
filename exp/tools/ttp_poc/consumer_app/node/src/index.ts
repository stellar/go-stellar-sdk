import { EventServiceClient } from './client';

async function main() {
    // an example of how to get a streamTTP events for a specific range of ledgers
    // from a GRPC serviceusing the protobufs generated code from github.comstellar/go/protos
    //
    const args = process.argv.slice(2);
    if (args.length !== 2) {
        console.error('Usage: node index.ts <startLedger> <endLedger>');
        process.exit(1);
    }

    const startLedger = parseInt(args[0]);
    // if the end ledger is less than the start ledger, the server will stream indefinitely
    const endLedger = parseInt(args[1]);

    if (isNaN(startLedger) || isNaN(endLedger)) {
        console.error('Error: startLedger and endLedger must be numbers');
        process.exit(1);
    }

    const client = new EventServiceClient();
    
    try {
        client.getTTPEvents(
            startLedger,
            endLedger,
            (event) => {
                // Do cool Stuff with TTP events!
                // ideally something more intresting than logging to console..
                console.log('Received TTP event:');
                console.log(JSON.stringify(event.toObject(), null, 2));
                console.log('-------------------');
            }
        );
        
    } catch (error) {
        console.error('Error getting TTP events:', error);
    }
}

main(); 