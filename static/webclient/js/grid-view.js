// webclient/js/grid-view.js
const GridView = {
    container: null,
    items: [],

    init(containerSelector) {
        this.container = document.querySelector(containerSelector);
        if (!this.container) return;

        this.setupViewListener();
        this.setupDataTableHook();
        this.render();
    },

    setupDataTableHook() {
        // Hook into DataTables init/success event
        const tryPopulate = () => {
            if (this.container && ViewToggle.getView() === 'grid') {
                this.populateFromDataTable();
            }
        };

        // Try immediately in case DataTable already initialized
        setTimeout(tryPopulate, 0);
        
        // Hook into DataTables draw event when dt becomes available
        const checkDt = setInterval(() => {
            if (typeof dt !== 'undefined' && dt.on) {
                dt.on('draw.dt', tryPopulate);
                clearInterval(checkDt);
                tryPopulate();
            }
        }, 100);
        
        // Also try again after a delay in case DataTable loads before we hook
        setTimeout(() => {
            clearInterval(checkDt);
            tryPopulate();
        }, 2000);
    },

    populateFromDataTable() {
        if (!this.container || ViewToggle.getView() !== 'grid') return;

        const rows = document.querySelectorAll('#file_manager_list_body tr');
        const items = [];

        rows.forEach(row => {
            const link = row.querySelector('a');
            if (!link) return;

            const name = link.textContent.trim();
            const href = link.getAttribute('href');
            const isDir = row.querySelector('.ki-folder') !== null;

            if (!isDir) {
                items.push({
                    name: name,
                    url: href,
                    thumb_cache_key: ''
                });
            }
        });

        if (items.length > 0) {
            this.setItems(items);
        }
    },

    setupViewListener() {
        document.addEventListener('viewChanged', (e) => {
            if (e.detail.view === 'grid') {
                this.show();
            } else {
                this.hide();
            }
        });
    },

    render() {
        if (ViewToggle.getView() === 'grid') {
            this.show();
        }
    },

    show() {
        if (this.container) {
            this.container.classList.remove('d-none');
        }
        this.populateFromDataTable();
        ThumbnailLoader.observeAll();
    },

    hide() {
        if (this.container) {
            this.container.classList.add('d-none');
        }
    },

    setItems(items) {
        this.items = items;
        this.renderItems();
    },

    renderItems() {
        if (!this.container || ViewToggle.getView() !== 'grid') return;

        this.container.innerHTML = this.items.map(item => this.createCellHTML(item)).join('');
        ThumbnailLoader.observeAll();
    },

    createCellHTML(item) {
        const filename = this.escapeHTML(item.name);
        const isImage = this.isImageFile(item.name);
        const cacheKey = item.thumb_cache_key || '';
        const url = item.url || '';

        if (!isImage) {
            return `
                <div class="thumbnail-cell" data-filename="${filename}">
                    <div class="error-state">
                        <i class="ki-duotone ki-file fs-2"></i>
                        <span class="fs-7">${filename}</span>
                    </div>
                </div>
            `;
        }

        return `
            <div class="thumbnail-cell" data-filename="${filename}" data-cache-key="${cacheKey}" data-url="${url}">
                <div class="skeleton"></div>
                <span class="filename">${filename}</span>
            </div>
        `;
    },


    isImageFile(filename) {
        const ext = filename.split('.').pop().toLowerCase();
        return ['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp'].includes(ext);
    },

    escapeHTML(str) {
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }
};

// Auto-initialize when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    GridView.init('.grid-view');
});