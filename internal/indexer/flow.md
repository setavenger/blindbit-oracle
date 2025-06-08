Process flow for indexing

We have to input streams for the indexer. 
1) Bitcoin-Kernel: to speed up initial indexing we attach directly to internal bitcoin core data 
2) RPC: the fallback and for continuous indexing 

The data outputs and formats are different which is why we need to unify them before pushing them into the indexing logic. Within the indexing logic there must not be any distinction between rpc or kernel data. We will use interfaces for input data

