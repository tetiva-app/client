-- Add gRPC metadata to collections for metadata inheritance
ALTER TABLE collections ADD COLUMN grpc_metadata TEXT NOT NULL DEFAULT '[]';
