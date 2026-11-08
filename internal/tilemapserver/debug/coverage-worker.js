/* GeoTIFF decoding stays off the map's main thread. */
importScripts('https://cdn.jsdelivr.net/npm/geotiff@2.1.3/dist-browser/geotiff.js');
let queue = Promise.resolve();
self.onmessage = ({data}) => {
    queue = queue.then(async () => {
        try {
            const tiff = await GeoTIFF.fromArrayBuffer(data.buffer);
            const image = await tiff.getImage();
            const width = image.getWidth(), height = image.getHeight();
            if (width > data.maxTextureSize || height > data.maxTextureSize || width * height > 16 * 1024 * 1024) {
                throw new Error('Gateway raster exceeds the browser texture limit');
            }
            const origin = image.getOrigin(), resolution = image.getResolution();
            if (![...origin.slice(0, 2), ...resolution.slice(0, 2)].every(Number.isFinite) || resolution[0] <= 0 || resolution[1] >= 0) {
                throw new Error('Unsupported raster geographic transform');
            }
            const bands = image.getSamplesPerPixel();
            if (![1, 2, 3, 4].includes(bands)) throw new Error('Unsupported raster band count');
            const alphaBand = bands === 4 ? 3 : bands === 2 ? 1 : null;
            const samples = await image.readRasters({samples: alphaBand === null ? [0] : [0, alphaBand]});
            const pixels = new Uint8Array(width * height * 2);
            for (let i = 0; i < width * height; i++) {
                // Signal-Server's gray palette is inverted: black is strongest.
                // Alpha is authoritative; legacy grayscale uses white as no-data.
                pixels[i * 2] = 255 - samples[0][i];
                pixels[i * 2 + 1] = alphaBand === null ? (samples[0][i] === 255 ? 0 : 255) : (samples[1][i] === 0 ? 0 : 255);
            }
            self.postMessage({id: data.id, width, height, origin: origin.slice(0, 2), resolution: resolution.slice(0, 2), pixels: pixels.buffer}, [pixels.buffer]);
        } catch (error) {
            self.postMessage({id: data.id, error: error.message});
        }
    });
};
