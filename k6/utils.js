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

export function stagesBuild(trace, builder) {
  switch (builder) {
    case "OneMinuteWindow":
      return stagesOneMinuteWindow(trace);

    case "TwoMinuteWindow":
      return stagesTwoMinuteWindow(trace);

    case "ThreeMinuteWindow":
      return stagesThreeMinuteWindow(trace);

    case "WithCooldown":
      return stagesWithCooldown(trace);

    default:
      throw new Error(`Unknown stage builder '${builder}'`);
  }
}

export function stagesOneMinuteWindow(trace) {
  let stages = [];

  for (const rate of trace) {
    const target = Math.round(rate);

    stages.push({
      duration: '5s', // 5-second transition to new rate.
      target,
    });
    stages.push({
      duration: '55s', // Keep a constant rate for the remainder of the minute.
      target,
    });
  }

  return stages;
}

export function stagesTwoMinuteWindow(trace) {
  let stages = [];

  for (const rate of trace) {
    const target = Math.round(rate);

    // Similar to stagesOneMinuteWindow() but with duration of 2 minutes.
    stages.push({
      duration: '10s',
      target,
    });
    stages.push({
      duration: '110s',
      target,
    });
  }

  return stages;
}

export function stagesThreeMinuteWindow(trace) {
  let stages = [];

  for (const rate of trace) {
    const target = Math.round(rate);

    // Similar to stagesOneMinuteWindow() but with duration of 3 minutes.
    stages.push({
      duration: '15s',
      target,
    });
    stages.push({
      duration: '165s',
      target,
    });
  }

  return stages;
}

export function stagesWithCooldown(trace) {
  let stages = [];

  for (const rate of trace) {
    const target = Math.round(rate);

    // Similar to stagesOneMinuteWindow(): an initial transition to new rate,
    // then constant rate.
    stages.push({
      duration: '20s',
      target,
    });
    stages.push({
      duration: '160s',
      target,
    });

    // Create a "cooldown" period to let the VM rest.
    stages.push({
      duration: '5s',
      target: 0,
    });
    stages.push({
      duration: '15s',
      target: 0,
    });
  }

  return stages;
}
