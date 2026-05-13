module sui_nexus::agent_memory {
    use sui::object::{Self, UID};
    use sui::tx_context::{Self, TxContext};
    use sui::transfer;
    use std::string::String;

    /// Stores AI agent context on-chain with Walrus blob reference
    public struct MemoryObject has key, store {
        id: UID,
        task_id: String,
        blob_id: String,
        agent_address: address,
        timestamp: u64,
    }

    /// Create a new memory object
    public fun create_memory(
        task_id: String,
        blob_id: String,
        ctx: &mut TxContext
    ) {
        let memory = MemoryObject {
            id: object::new(ctx),
            task_id,
            blob_id,
            agent_address: tx_context::sender(ctx),
            timestamp: tx_context::epoch(ctx),
        };
        transfer::public_transfer(memory, tx_context::sender(ctx));
    }
}
