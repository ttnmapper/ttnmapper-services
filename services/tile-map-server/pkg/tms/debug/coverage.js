/* Progressive native raster coverage, with WebGL2 and Canvas rendering. */
(() => {
    'use strict';
    const MEMORY_LIMIT = 64 * 1024 * 1024;
    const DOWNLOAD_LIMIT = 64 * 1024 * 1024;
    const VERTEX = `#version 300 es
        in vec2 position;
        void main() { gl_Position = vec4(position, 0., 1.); }`;
    const MERGE = `#version 300 es
        precision highp float;
        uniform sampler2D raster;
        uniform vec4 view;
        uniform vec2 screen;
        uniform vec4 geo;
        uniform ivec2 dimensions;
        out vec4 outputSignal;
        void main() {
            vec2 point = vec2(gl_FragCoord.x / screen.x, 1. - gl_FragCoord.y / screen.y);
            vec2 world = view.xy + point * view.zw;
            float lon = world.x * 360. - 180.;
            float center = geo.x + float(dimensions.x) * geo.z * .5;
            lon += 360. * floor((center - lon) / 360. + .5);
            float lat = atan(sinh(3.141592653589793 * (1. - 2. * world.y))) * 57.29577951308232;
            ivec2 cell = ivec2(floor(vec2((lon - geo.x) / geo.z, (geo.y - lat) / geo.w) + .5));
            if (any(lessThan(cell, ivec2(0))) || any(greaterThanEqual(cell, dimensions))) discard;
            vec2 sampleValue = texelFetch(raster, cell, 0).rg;
            if (sampleValue.g < .5) discard;
            outputSignal = vec4(sampleValue.r, 0., 0., 1.);
        }`;
    const COLOR = `#version 300 es
        precision highp float;
        uniform sampler2D maximum;
        uniform float opacity;
        out vec4 outputColor;
        void main() {
            vec4 value = texelFetch(maximum, ivec2(gl_FragCoord.xy), 0);
            if (value.a == 0.) { outputColor = vec4(0.); return; }
            float dbm = -150. + value.r * 150.;
            vec3 color = dbm > -100. ? vec3(1.,0.,0.) :
                dbm > -105. ? vec3(1.,127./255.,0.) :
                dbm > -110. ? vec3(1.,1.,0.) :
                dbm > -115. ? vec3(0.,1.,0.) :
                dbm > -120. ? vec3(0.,1.,1.) : vec3(0.,0.,1.);
            outputColor = vec4(color, opacity);
        }`;

    window.SimCovRasterLayer = L.Layer.extend({
        initialize(options) { L.setOptions(this, options); },
        onAdd(map) {
            this._map = map;
            this._records = new Map();
            this._generation = (this._generation || 0) + 1;
            this._gl = null; this._cpu = null; this._worker = null; this._workerFailed = false;
            this._requests = new Map();
            this._nextID = 0;
            this._renderGeneration = 0;
            this._canvas = L.DomUtil.create('canvas', 'simcov-canvas');
            this._canvas.width = this._canvas.height = 0;
            this._zooming = false;
            this._moving = false;
            this._canvas.style.cssText = `position:absolute;pointer-events:none;z-index:${this.options.zIndex || 11}`;
            map.getPane('overlayPane').appendChild(this._canvas);
            this._status = L.control({position: 'bottomleft'});
            this._status.onAdd = () => {
                this._statusElement = L.DomUtil.create('div', 'coverage-status');
                L.DomEvent.disableClickPropagation(this._statusElement);
                return this._statusElement;
            };
            this._status.addTo(map);
            try {
                this._initGL();
                this._createWorker();
                map.on('movestart', this._startMove, this);
                map.on('resize', this._scheduleRender, this);
                map.on('moveend', this._refresh, this);
                map.on('zoomstart', this._hide, this);
                this._refresh();
            } catch (error) { this._message(error.message); }
        },
        onRemove(map) {
            ++this._generation;
            ++this._renderGeneration;
            if (this._controller) this._controller.abort();
            cancelAnimationFrame(this._frame);
            map.off('movestart', this._startMove, this);
            map.off('resize', this._scheduleRender, this);
            map.off('moveend', this._refresh, this);
            map.off('zoomstart', this._hide, this);
            if (this._worker) this._worker.terminate();
            for (const request of this._requests.values()) request.reject(new DOMException('Layer removed', 'AbortError'));
            this._requests.clear();
            if (this._gl) {
                for (const record of this._records.values()) this._gl.deleteTexture(record.texture);
                this._gl.deleteTexture(this._maximum);
                this._gl.deleteFramebuffer(this._framebuffer);
                this._gl.deleteBuffer(this._quad);
                this._gl.deleteVertexArray(this._vao);
                this._gl.deleteProgram(this._merge);
                this._gl.deleteProgram(this._color);
            }
            this._canvas.remove();
            this._status.remove();
            this._map = null;
        },
        _createWorker() {
            this._workerFailed = false;
            const worker = this._worker = new Worker('/debug/assets/coverage-worker.js');
            worker.onmessage = ({data}) => {
                if (this._worker !== worker) return;
                const request = this._requests.get(data.id);
                if (!request) return;
                this._requests.delete(data.id);
                data.error ? request.reject(new Error(data.error)) : request.resolve(data);
            };
            worker.onerror = () => {
                if (this._worker !== worker) return;
                this._workerFailed = true;
                for (const request of this._requests.values()) request.reject(new Error('GeoTIFF decoder could not load'));
                this._requests.clear();
                this._message('GeoTIFF decoder could not load; reload to retry');
            };
        },
        _message(message) {
            this._statusElement.textContent = `${this.options.gatewayId ? 'Gateway' : 'Network'} coverage: ${message}${this._cpu ? ' · CPU rendering' : ''}`;
        },
        _startMove() {
            this._moving = true;
            ++this._renderGeneration;
            cancelAnimationFrame(this._frame);
            // Keep the finished canvas anchored in the overlay pane. Leaflet
            // translates that pane with the base map throughout a pan.
        },
        _hide() { ++this._renderGeneration; this._zooming = true; this._canvas.style.visibility = 'hidden'; },
        _program(fragment) {
            const gl = this._gl, program = gl.createProgram();
            for (const [type, source] of [[gl.VERTEX_SHADER, VERTEX], [gl.FRAGMENT_SHADER, fragment]]) {
                const shader = gl.createShader(type);
                gl.shaderSource(shader, source); gl.compileShader(shader);
                if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) throw new Error(gl.getShaderInfoLog(shader));
                gl.attachShader(program, shader); gl.deleteShader(shader);
            }
            gl.linkProgram(program);
            if (!gl.getProgramParameter(program, gl.LINK_STATUS)) throw new Error(gl.getProgramInfoLog(program));
            return program;
        },
        _initGL() {
            const gl = this._canvas.getContext('webgl2', {alpha: true, premultipliedAlpha: false, depth: false, antialias: false});
            if (!gl) {
                this._cpu = this._canvas.getContext('2d');
                if (!this._cpu) throw new Error('Coverage canvas unavailable');
                return;
            }
            this._gl = gl;
            this._merge = this._program(MERGE);
            this._color = this._program(COLOR);
            this._vao = gl.createVertexArray(); gl.bindVertexArray(this._vao);
            this._quad = gl.createBuffer(); gl.bindBuffer(gl.ARRAY_BUFFER, this._quad);
            gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1,-1, 1,-1, -1,1, -1,1, 1,-1, 1,1]), gl.STATIC_DRAW);
            for (const program of [this._merge, this._color]) {
                const position = gl.getAttribLocation(program, 'position');
                gl.enableVertexAttribArray(position); gl.vertexAttribPointer(position, 2, gl.FLOAT, false, 0, 0);
            }
            this._maximum = gl.createTexture();
            this._framebuffer = gl.createFramebuffer();
            this._canvas.addEventListener('webglcontextlost', event => {
                event.preventDefault();
                if (this._controller) this._controller.abort();
                this._message('graphics context lost; reload to retry');
            });
        },
        _textureParameters() {
            const gl = this._gl;
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
        },
        _decode(buffer, signal) {
            if (signal.aborted) return Promise.reject(new DOMException('View changed', 'AbortError'));
            if (this._workerFailed) return Promise.reject(new Error('GeoTIFF decoder unavailable'));
            return new Promise((resolve, reject) => {
                const id = ++this._nextID;
                const abort = () => { this._requests.delete(id); reject(new DOMException('View changed', 'AbortError')); };
                signal.addEventListener('abort', abort, {once: true});
                const finish = fn => value => { signal.removeEventListener('abort', abort); fn(value); };
                this._requests.set(id, {resolve: finish(resolve), reject: finish(reject)});
                this._worker.postMessage({id, buffer, maxTextureSize: this._gl ? this._gl.getParameter(this._gl.MAX_TEXTURE_SIZE) : 16384}, [buffer]);
            });
        },
        async _fetchRaster(url, signal) {
            const response = await fetch(url, {signal});
            if (!response.ok) throw new Error(`Raster request failed (${response.status})`);
            const reader = response.body.getReader(), chunks = [];
            let size = 0;
            while (true) {
                const {done, value} = await reader.read();
                if (done) break;
                size += value.byteLength;
                if (size > DOWNLOAD_LIMIT) { await reader.cancel(); throw new Error('Gateway raster download exceeds 64 MB'); }
                chunks.push(value);
            }
            const bytes = new Uint8Array(size);
            let offset = 0;
            for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.byteLength; }
            return this._decode(bytes.buffer, signal);
        },
        _upload(url, raster) {
            const gl = this._gl, bytes = raster.width * raster.height * 2;
            let used = [...this._records.values()].reduce((sum, record) => sum + record.bytes, 0);
            // Evict only rasters outside the current manifest, retaining all
            // contributors needed to reconstruct maximum after a pan or removal.
            for (const [key, record] of this._records) {
                if (used + bytes <= MEMORY_LIMIT) break;
                if (!this._visible.has(key)) { if (gl) gl.deleteTexture(record.texture); this._records.delete(key); used -= record.bytes; }
            }
            if (used + bytes > MEMORY_LIMIT) throw new Error('Visible rasters exceed 64 MB; zoom in');
            if (!gl) {
                this._records.set(url, {bytes, width: raster.width, height: raster.height,
                    origin: raster.origin, resolution: raster.resolution, pixels: new Uint8Array(raster.pixels)});
                return;
            }
            const texture = gl.createTexture(); gl.bindTexture(gl.TEXTURE_2D, texture);
            this._textureParameters(); gl.pixelStorei(gl.UNPACK_ALIGNMENT, 1);
            gl.texImage2D(gl.TEXTURE_2D, 0, gl.RG8, raster.width, raster.height, 0, gl.RG, gl.UNSIGNED_BYTE, new Uint8Array(raster.pixels));
            if (gl.getError() !== gl.NO_ERROR) { gl.deleteTexture(texture); throw new Error('Could not upload gateway raster'); }
            this._records.set(url, {texture, bytes, width: raster.width, height: raster.height, origin: raster.origin, resolution: raster.resolution});
        },
        async _refresh() {
            if (!this._map || (!this._gl && !this._cpu) || this._gl?.isContextLost()) return;
            this._zooming = false;
            this._moving = false;
            const generation = ++this._generation;
            const decoding = this._requests?.size > 0;
            if (this._controller) this._controller.abort();
            // Stop superseded decodes, rather than accumulating old buffers in
            // the worker's queue during rapid map movements.
            if (decoding || this._workerFailed) {
                if (this._worker) this._worker.terminate();
                this._createWorker();
            }
            this._controller = new AbortController();
            const signal = this._controller.signal;
            // Keep the previous contributors visible while the new manifest
            // is in flight; replace them when its current-view list arrives.
            this._visible = this._visible || new Set();
            this._scheduleRender();
            this._message('finding gateways…');
            const bounds = this._map.getBounds();
            const span = bounds.getEast() - bounds.getWest();
            const wrap = value => ((value + 180) % 360 + 360) % 360 - 180;
            const query = new URLSearchParams({
                west: span >= 360 ? -180 : wrap(bounds.getWest()),
                east: span >= 360 ? 180 : wrap(bounds.getEast()),
                south: Math.max(-85.051128, bounds.getSouth()), north: Math.min(85.051128, bounds.getNorth())
            });
            if (this.options.gatewayId) query.set('gateway_id', this.options.gatewayId);
            try {
                const response = await fetch(`/simcov/rasters/network/${encodeURIComponent(this.options.networkId)}?${query}`, {signal});
                if (!response.ok) throw new Error((await response.text()).trim());
                const manifest = await response.json();
                if (signal.aborted || generation !== this._generation) return;
                this._visible = new Set(manifest.gateways.map(gateway => gateway.url));
                let loaded = manifest.gateways.filter(gateway => this._records.has(gateway.url)).length;
                let failed = 0, next = 0, lastError = '';
                const progress = () => {
                    if (signal.aborted || generation !== this._generation) return;
                    this._message(`${loaded}/${manifest.gateways.length} loaded${failed ? `, ${failed} failed (${lastError})` : ''}${loaded + failed < manifest.gateways.length ? ' · partial' : failed ? ' · incomplete' : ' · complete'}`);
                    this._scheduleRender();
                };
                progress();
                const pending = manifest.gateways.filter(gateway => !this._records.has(gateway.url)).sort((a, b) => Number(b.cached) - Number(a.cached));
                const load = async () => {
                    while (next < pending.length && !signal.aborted) {
                        const gateway = pending[next++];
                        try {
                            const raster = await this._fetchRaster(gateway.url, signal);
                            if (signal.aborted || generation !== this._generation) return;
                            this._upload(gateway.url, raster); loaded++;
                        } catch (error) {
                            if (error.name === 'AbortError') return;
                            failed++; lastError = error.message;
                        }
                        progress();
                    }
                };
                await Promise.all([load(), load()]);
            } catch (error) {
                if (error.name !== 'AbortError' && generation === this._generation) this._message(error.message);
            }
        },
        _scheduleRender() {
            if (this._moving) return;
            this._renderGeneration = (this._renderGeneration || 0) + 1;
            cancelAnimationFrame(this._frame);
            this._frame = requestAnimationFrame(() => this._render());
        },
        _renderCPU() {
            const size = this._map.getSize();
            const ratio = Math.min(1, Math.sqrt(1000000 / Math.max(1, size.x * size.y)));
            const width = Math.max(1, Math.floor(size.x * ratio)), height = Math.max(1, Math.floor(size.y * ratio));
            const position = this._map.containerPointToLayerPoint([0, 0]);
            const zoom = this._map.getZoom(), worldSize = 256 * Math.pow(2, zoom);
            const topLeft = this._map.project(this._map.containerPointToLatLng([0, 0]), zoom);
            const image = this._cpu.createImageData(width, height);
            const records = [];
            for (const url of this._visible || []) {
                const record = this._records.get(url);
                if (!record) continue;
                const columns = new Int32Array(width);
                const center = record.origin[0] + record.width * record.resolution[0] * .5;
                for (let x = 0; x < width; x++) {
                    let lon = (topLeft.x + (x + .5) * size.x / width) / worldSize * 360 - 180;
                    lon += 360 * Math.floor((center - lon) / 360 + .5);
                    columns[x] = Math.floor((lon - record.origin[0]) / record.resolution[0] + .5);
                }
                records.push({record, columns});
            }
            const generation = this._renderGeneration = (this._renderGeneration || 0) + 1;
            const alpha = Math.round(255 * (this.options.opacity ?? .5));
            let y = 0;
            const draw = () => {
                if (generation !== this._renderGeneration || !this._map || this._zooming || this._moving) return;
                const started = performance.now();
                do {
                    const worldY = (topLeft.y + (y + .5) * size.y / height) / worldSize;
                    const lat = Math.atan(Math.sinh(Math.PI * (1 - 2 * worldY))) * 180 / Math.PI;
                    const maximum = new Int16Array(width); maximum.fill(-1);
                    for (const {record, columns} of records) {
                        const row = Math.floor((record.origin[1] - lat) / -record.resolution[1] + .5);
                        if (row < 0 || row >= record.height) continue;
                        const offset = row * record.width * 2;
                        for (let x = 0; x < width; x++) {
                            const column = columns[x];
                            if (column < 0 || column >= record.width) continue;
                            const index = offset + column * 2;
                            if (record.pixels[index + 1] && record.pixels[index] > maximum[x]) maximum[x] = record.pixels[index];
                        }
                    }
                    for (let x = 0; x < width; x++) {
                        if (maximum[x] < 0) continue;
                        const dbm = -150 + maximum[x] * 150 / 255;
                        const index = (y * width + x) * 4;
                        image.data[index] = dbm > -110 ? 255 : 0;
                        image.data[index + 1] = dbm > -100 ? 0 : dbm > -105 ? 127 : dbm > -120 ? 255 : 0;
                        image.data[index + 2] = dbm <= -115 ? 255 : 0;
                        image.data[index + 3] = alpha;
                    }
                    y++;
                } while (y < height && performance.now() - started < 8);
                if (y < height) this._frame = requestAnimationFrame(draw);
                else {
                    // Publish pixels and their map position together. Moving
                    // the old image first would leave it pinned to the screen
                    // until this time-sliced composition finishes.
                    if (this._canvas.width !== width || this._canvas.height !== height) {
                        this._canvas.width = width; this._canvas.height = height;
                    }
                    this._cpu.putImageData(image, 0, 0);
                    L.DomUtil.setPosition(this._canvas, position);
                    this._canvas.style.width = `${size.x}px`; this._canvas.style.height = `${size.y}px`;
                    this._canvas.style.visibility = '';
                }
            };
            draw();
        },
        _render() {
            if (!this._map || this._zooming || this._moving || this._gl?.isContextLost()) return;
            if (this._cpu) { this._renderCPU(); return; }
            if (!this._gl) return;
            const gl = this._gl, size = this._map.getSize(), ratio = Math.min(2, window.devicePixelRatio || 1);
            const width = Math.max(1, Math.round(size.x * ratio)), height = Math.max(1, Math.round(size.y * ratio));
            L.DomUtil.setPosition(this._canvas, this._map.containerPointToLayerPoint([0, 0]));
            this._canvas.style.width = `${size.x}px`; this._canvas.style.height = `${size.y}px`;
            this._canvas.style.visibility = '';
            if (this._canvas.width !== width || this._canvas.height !== height) {
                this._canvas.width = width; this._canvas.height = height;
                gl.bindTexture(gl.TEXTURE_2D, this._maximum); this._textureParameters();
                gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, width, height, 0, gl.RGBA, gl.UNSIGNED_BYTE, null);
                gl.bindFramebuffer(gl.FRAMEBUFFER, this._framebuffer);
                gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, this._maximum, 0);
                if (gl.checkFramebufferStatus(gl.FRAMEBUFFER) !== gl.FRAMEBUFFER_COMPLETE) { this._message('Coverage framebuffer unavailable'); return; }
            }
            const zoom = this._map.getZoom(), worldSize = 256 * Math.pow(2, zoom);
            const topLeft = this._map.project(this._map.containerPointToLatLng([0, 0]), zoom);
            gl.bindVertexArray(this._vao); gl.viewport(0, 0, width, height);
            gl.activeTexture(gl.TEXTURE0); gl.bindFramebuffer(gl.FRAMEBUFFER, this._framebuffer);
            gl.clearColor(0,0,0,0); gl.clear(gl.COLOR_BUFFER_BIT);
            gl.useProgram(this._merge); gl.enable(gl.BLEND); gl.blendEquation(gl.MAX);
            gl.uniform1i(gl.getUniformLocation(this._merge, 'raster'), 0);
            gl.uniform4f(gl.getUniformLocation(this._merge, 'view'), topLeft.x / worldSize, topLeft.y / worldSize, size.x / worldSize, size.y / worldSize);
            gl.uniform2f(gl.getUniformLocation(this._merge, 'screen'), width, height);
            for (const url of this._visible || []) {
                const record = this._records.get(url);
                if (!record) continue;
                gl.bindTexture(gl.TEXTURE_2D, record.texture);
                gl.uniform4f(gl.getUniformLocation(this._merge, 'geo'), record.origin[0], record.origin[1], record.resolution[0], -record.resolution[1]);
                gl.uniform2i(gl.getUniformLocation(this._merge, 'dimensions'), record.width, record.height);
                gl.drawArrays(gl.TRIANGLES, 0, 6);
            }
            gl.disable(gl.BLEND); gl.bindFramebuffer(gl.FRAMEBUFFER, null);
            gl.useProgram(this._color); gl.bindTexture(gl.TEXTURE_2D, this._maximum);
            gl.uniform1i(gl.getUniformLocation(this._color, 'maximum'), 0);
            gl.uniform1f(gl.getUniformLocation(this._color, 'opacity'), this.options.opacity ?? .5);
            gl.drawArrays(gl.TRIANGLES, 0, 6);
        }
    });
})();
