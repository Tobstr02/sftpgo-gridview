// webclient/js/view-toggle.js
const ViewToggle = {
    STORAGE_KEY: 'filesViewMode',
    VIEW_LIST: 'list',
    VIEW_GRID: 'grid',
    currentView: null,


    init() {
        this.currentView = localStorage.getItem(this.STORAGE_KEY) || this.VIEW_GRID;
        this.render();
        this.bindEvents();
        
        // Dispatch initial view change event so other components can react
        document.dispatchEvent(new CustomEvent('viewChanged', { detail: { view: this.currentView } }));
    },

    render() {
        const container = document.getElementById('view-toggle');
        if (!container) return;


        container.innerHTML = `
            <div class="btn-group" role="group">
                <button type="button" class="btn btn-icon ${this.currentView === this.VIEW_LIST ? 'btn-primary' : 'btn-light'}" 
                        data-view="${this.VIEW_LIST}" title="List view">
                    <i class="ki-duotone ki-row-vertical fs-2"></i>
                </button>
                <button type="button" class="btn btn-icon ${this.currentView === this.VIEW_GRID ? 'btn-primary' : 'btn-light'}" 
                        data-view="${this.VIEW_GRID}" title="Grid view">
                    <i class="ki-duotone ki-grid fs-2"></i>
                </button>
            </div>
        `;
    },


    bindEvents() {
        const container = document.getElementById('view-toggle');
        if (!container) return;


        container.addEventListener('click', (e) => {
            const btn = e.target.closest('[data-view]');
            if (!btn) return;
            this.setView(btn.dataset.view);
        });
    },


    setView(view) {
        if (this.currentView === view) return;
        this.currentView = view;
        localStorage.setItem(this.STORAGE_KEY, view);
        this.render();
        document.dispatchEvent(new CustomEvent('viewChanged', { detail: { view } }));
    },

    getView() {
        return this.currentView;
    }
};

document.addEventListener('DOMContentLoaded', () => ViewToggle.init());