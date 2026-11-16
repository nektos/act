if (process.env.INPUT_CHILD_VALUE !== "from-workflow") {
  throw new Error(`unexpected child input: ${process.env.INPUT_CHILD_VALUE}`);
}

for (const name of ["INPUT_EXPLICIT", "INPUT_DERIVED", "INPUT_ENABLED"]) {
  if (name in process.env) {
    throw new Error(`parent composite input leaked as ${name}`);
  }
}
