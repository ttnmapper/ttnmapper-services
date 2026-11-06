const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');

function layerPrototype(fetch, extra = {}) {
    const window = {};
    const L = {Layer: {extend(prototype) { function Layer() {} Layer.prototype = prototype; return Layer; }}};
    vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'coverage.js'), 'utf8'), {
        window, L, fetch, URLSearchParams, AbortController, DOMException, Map, Set, Uint8Array, Float32Array,
        Int16Array, Int32Array, performance, ...extra,
    });
    return window.SimCovRasterLayer.prototype;
}

test('worker preserves signal and validity as separate numeric channels', async () => {
    let finish;
    const done = new Promise(resolve => { finish = resolve; });
    const self = {postMessage: finish};
    const GeoTIFF = {fromArrayBuffer: async () => ({getImage: async () => ({
        getWidth: () => 4, getHeight: () => 1, getOrigin: () => [4, 52, 0],
        getResolution: () => [.001, -.001, 0], getSamplesPerPixel: () => 4,
        readRasters: async () => [new Uint8Array([190, 230, 0, 255]), new Uint8Array([255, 255, 0, 255])],
    })})};
    vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'coverage-worker.js'), 'utf8'), {
        self, GeoTIFF, importScripts() {}, Uint8Array, Promise, Number, Error,
    });
    self.onmessage({data: {id: 1, buffer: new ArrayBuffer(0), maxTextureSize: 4096}});
    const output = await done;
    assert.equal(output.error, undefined);
    assert.deepEqual([...new Uint8Array(output.pixels)], [65,255,25,255,255,0,0,255]);
});

test('cached rasters load first and progress appears before all gateways finish', async () => {
    const manifest = {gateways: [
        {url: 'slow', cached: false}, {url: 'cached', cached: true}, {url: 'third', cached: false},
    ]};
    const prototype = layerPrototype(async () => ({ok: true, json: async () => manifest}));
    const layer = Object.create(prototype);
    layer.options = {networkId: 'net'};
    layer._map = {getBounds: () => ({getWest: () => 3, getEast: () => 5, getSouth: () => 51, getNorth: () => 53})};
    layer._gl = {isContextLost: () => false};
    layer._generation = 0;
    layer._records = new Map();
    const messages = [], started = [], pending = new Map();
    layer._message = message => messages.push(message);
    layer._scheduleRender = () => {};
    layer._upload = url => layer._records.set(url, {});
    layer._fetchRaster = url => {
        started.push(url);
        return new Promise(resolve => pending.set(url, resolve));
    };
    const done = layer._refresh();
    await new Promise(resolve => setImmediate(resolve));
    assert.deepEqual(started, ['cached', 'slow']);
    pending.get('cached')({});
    await new Promise(resolve => setImmediate(resolve));
    assert.ok(messages.some(message => message.includes('1/3 loaded') && message.includes('partial')));
    assert.deepEqual(started, ['cached', 'slow', 'third']);
    pending.get('third')({}); pending.get('slow')({});
    await done;
    assert.equal(messages.at(-1), '3/3 loaded · complete');
});

test('failed gateways are reported as incomplete coverage', async () => {
    const prototype = layerPrototype(async () => ({ok: true, json: async () => ({gateways: [{url:'failed', cached:false}]})}));
    const layer = Object.create(prototype);
    layer.options = {networkId:'net'};
    layer._map = {getBounds: () => ({getWest:()=>3,getEast:()=>5,getSouth:()=>51,getNorth:()=>53})};
    layer._gl = {isContextLost:()=>false}; layer._generation=0; layer._records=new Map();
    const messages=[];
    layer._message=message=>messages.push(message); layer._scheduleRender=()=>{};
    layer._fetchRaster=async()=>{throw new Error('simulation failed');};
    await layer._refresh();
    assert.equal(messages.at(-1),'0/1 loaded, 1 failed (simulation failed) · incomplete');
});


test('missing WebGL2 uses Canvas without blocking progressive loading', async () => {
    const prototype = layerPrototype(async () => ({ok: true, json: async () => ({gateways: [{url: 'gateway', cached: true}]})}));
    const layer = Object.create(prototype), context = {};
    layer._canvas = {getContext: type => type === '2d' ? context : null};
    layer._initGL();
    assert.equal(layer._cpu, context);
    layer.options = {networkId: 'net'};
    layer._map = {getBounds: () => ({getWest:()=>3,getEast:()=>5,getSouth:()=>51,getNorth:()=>53})};
    layer._generation = 0; layer._records = new Map();
    const messages = [];
    layer._message = message => messages.push(message); layer._scheduleRender = () => {};
    layer._fetchRaster = async () => ({width:1,height:1,origin:[4,52],resolution:[1,-1],pixels:new Uint8Array([65,255]).buffer});
    await layer._refresh();
    assert.equal(messages.at(-1), '1/1 loaded · complete');
    assert.deepEqual([...layer._records.get('gateway').pixels], [65,255]);
});

test('Canvas composes strongest valid signal regardless of arrival order, including wrapped longitude', () => {
    const prototype = layerPrototype(undefined, {L: {
        Layer: {extend(p) {function Layer() {} Layer.prototype=p;return Layer;}},
        DomUtil: {setPosition() {}},
    }});
    const layer = Object.create(prototype);
    layer.options = {opacity: .5};
    layer._canvas = {style:{}};
    layer._map = {getSize:()=>({x:3,y:1}),containerPointToLayerPoint:()=>[0,0],
        containerPointToLatLng:()=>[0,0],getZoom:()=>0,project:()=>({x:0,y:128})};
    let image;
    layer._cpu = {createImageData:(w,h)=>({data:new Uint8Array(w*h*4)}),putImageData:value=>{image=value;}};
    const raster = pixels => ({width:3,height:1,origin:[540 + 180/256,0],resolution:[360/256,-2],pixels:new Uint8Array(pixels)});
    layer._records = new Map([
        ['weak',raster([25,255,0,255,255,0])], ['strong',raster([65,255,255,0,255,0])],
    ]);
    layer._visible = new Set(['weak','strong']); layer._renderCPU();
    const expected = [0,255,0,128, 0,0,255,128, 0,0,0,0];
    assert.deepEqual([...image.data], expected);
    layer._visible = new Set(['strong','weak']); layer._renderCPU();
    assert.deepEqual([...image.data], expected);
    layer._visible = new Set(['weak']); layer._renderCPU();
    assert.deepEqual([...image.data.slice(0,4)], [0,0,255,128]);
});

test('CPU redraw keeps old pixels and position until completion, and a pan cancels it', () => {
    const queued = [], published = [], positions = [];
    let clock = 0;
    const prototype = layerPrototype(undefined, {
        performance: {now: () => { clock += 10; return clock; }},
        requestAnimationFrame: fn => {queued.push(fn); return queued.length;},
        cancelAnimationFrame() {},
        L: {Layer: {extend(p) {function Layer() {} Layer.prototype=p;return Layer;}},
            DomUtil: {setPosition: (_canvas, position) => positions.push(position)}},
    });
    const layer = Object.create(prototype);
    layer.options = {}; layer._generation = 0; layer._renderGeneration = 0;
    layer._canvas = {width:7,height:7,style:{}};
    layer._map = {getSize:()=>({x:3,y:3}),containerPointToLayerPoint:()=>[40,20],
        containerPointToLatLng:()=>[0,0],getZoom:()=>0,project:()=>({x:0,y:128})};
    layer._records = new Map(); layer._visible = new Set();
    layer._cpu = {createImageData:(w,h)=>({data:new Uint8Array(w*h*4)}),putImageData:image=>published.push(image)};
    layer._renderCPU();
    assert.equal(published.length, 0); assert.equal(positions.length, 0);
    assert.equal(layer._canvas.width, 7);
    layer._startMove();
    queued.shift()();
    layer._scheduleRender();
    assert.equal(queued.length, 0); assert.equal(published.length, 0); assert.equal(positions.length, 0);
    layer._moving = false; layer._renderCPU();
    while (queued.length) queued.shift()();
    assert.equal(published.length, 1); assert.deepEqual(positions, [[40,20]]);
    assert.equal(layer._canvas.width, 3);
});

test('refresh retains coverage until the new view manifest arrives', async () => {
    let resolveManifest;
    const response = new Promise(resolve => {resolveManifest=resolve;});
    const prototype = layerPrototype(async () => response);
    const layer = Object.create(prototype);
    layer.options = {networkId:'net'}; layer._generation=0; layer._gl={isContextLost:()=>false};
    layer._map={getBounds:()=>({getWest:()=>3,getEast:()=>5,getSouth:()=>51,getNorth:()=>53})};
    layer._records=new Map([['existing',{}]]); layer._visible=new Set(['existing']);
    layer._scheduleRender=()=>{}; layer._message=()=>{};
    const done=layer._refresh();
    assert.deepEqual([...layer._visible], ['existing']);
    resolveManifest({ok:true,json:async()=>({gateways:[]})});
    await done;
    assert.equal(layer._visible.size, 0);
});
