import { open } from 'k6/experimental/fs';

// See: https://grafana.com/docs/k6/latest/javascript-api/k6-experimental/fs/file/read/#readall-helper-function
export async function readAll(path) {
  const file = await open(path)
  const fileInfo = await file.stat();
  const buffer = new Uint8Array(fileInfo.size);

  const bytesRead = await file.read(buffer);
  if (bytesRead !== fileInfo.size) {
    throw new Error(
      'unexpected number of bytes read; expected ' +
      fileInfo.size +
      ' but got ' +
      bytesRead +
      ' bytes'
    );
  }

  return buffer;
}
