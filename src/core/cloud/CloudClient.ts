export class CloudClient {
  constructor(private endpoint: string, private apiKey: string) {}

  public async uploadChunk(filename: string, content: string): Promise<void> {
    // In a real implementation this would use fetch to upload the file to a cloud service.
    // For local simulation, we just resolve.
    return Promise.resolve();
  }

  public async downloadChunks(since: string): Promise<{filename: string, content: string}[]> {
    return Promise.resolve([]);
  }
}
