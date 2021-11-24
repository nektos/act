console.log('pre step');

if (process.env.INPUT_FAILINPRESTEP === 'true') {
    throw new Error("Fail in pre step");
}
