console.log('post step');

if (process.env.INPUT_FAILINPOSTSTEP === 'true') {
    throw new Error("Fail in post step");
}
