// webclient/js/thumbnail-loader.js
const ThumbnailLoader = {
    observer: null,
    thumbURL: '/thumb/generate',
    cacheURL: '/thumb/',


    init() {
        this.setupIntersectionObserver();
    },


    setupIntersectionObserver() {
        this.observer = new IntersectionObserver(
            (entries) => {
                entries.forEach(entry => {
                    if (entry.isIntersecting) {
                        this.loadThumbnail(entry.target);
                        this.observer.unobserve(entry.target);
                    }
                });
            },
            { rootMargin: '200px' }
        );
    },

    observeAll() {
        const cells = document.querySelectorAll('.thumbnail-cell[data-cache-key]');
        cells.forEach(cell => {
            if (!cell.querySelector('img[src]') && !cell.querySelector('.error-state')) {
                this.observer.observe(cell);
            }
        });
    },

    loadThumbnail(cell) {
        const key = cell.dataset.thumbKey;
        const cacheKey = cell.dataset.cacheKey;
        const url = cell.dataset.url;

        if (cacheKey) {
            this.showImage(cell, this.cacheURL + cacheKey);
            return;
        }

        this.requestGeneration(cell, url, key);
    },


    showImage(cell, src) {
        const skeleton = cell.querySelector('.skeleton');
        if (skeleton) skeleton.remove();

        let img = cell.querySelector('img');
        if (!img) {
            img = document.createElement('img');
            img.alt = cell.dataset.filename || 'Thumbnail';
            cell.appendChild(img);
        }

        img.onload = () => cell.classList.add('loaded');
        img.onerror = () => this.showError(cell, 'Failed to load image');
        img.src = src;
    },

    showError(cell, message) {
        cell.innerHTML = `
            <div class="error-state">
                <i class="ki-duotone ki-warning text-danger fs-2"></i>
                <span class="fs-7">${message}</span>
            </div>
        `;
    },


    async requestGeneration(cell, fileURL, key) {
        try {
            const response = await fetch(this.thumbURL, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-CSRF-TOKEN': window.csrfToken
                },
                body: JSON.stringify({ url: fileURL })
            });


            if (!response.ok) {
                this.showError(cell, 'Generation failed');
                return;
            }

            const data = await response.json();
            cell.dataset.cacheKey = data.cache_key;
            this.showImage(cell, this.cacheURL + data.cache_key);
        } catch (err) {
            this.showError(cell, 'Request failed');
        }
    }
};

document.addEventListener('DOMContentLoaded', () => ThumbnailLoader.init());