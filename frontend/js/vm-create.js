document.addEventListener('DOMContentLoaded', function() {
    const tagInput = document.getElementById('tagInput');
    const addTagBtn = document.getElementById('addTagBtn');
    const tagsContainer = document.getElementById('tagsContainer');
    const tagsInput = document.getElementById('tagsInput');
    let tags = [];

    // Add tag function
    function addTag() {
        const tagText = tagInput.value.trim();
        
        if (tagText && !tags.includes(tagText)) {
            tags.push(tagText);
            updateTags();
            tagInput.value = '';
        }
        
        tagInput.focus();
    }

    // Update tags display and hidden input
    function updateTags() {
        // Clear container
        tagsContainer.innerHTML = '';
        
        // Add each tag as a Bulma tag
        tags.forEach((tag, index) => {
            const tagElement = document.createElement('span');
            tagElement.className = 'tag is-primary is-light is-medium';
            tagElement.innerHTML = `
                ${tag}
                <button class="delete is-small" data-index="${index}"></button>
            `;
            tagsContainer.appendChild(tagElement);
        });
        
        // Update hidden input with comma-separated tags
        tagsInput.value = tags.join(',');
        
        // Add event listeners to delete buttons
        document.querySelectorAll('.delete').forEach(button => {
            button.addEventListener('click', function() {
                const index = parseInt(this.getAttribute('data-index'));
                tags.splice(index, 1);
                updateTags();
            });
        });
    }

    // Add tag on button click
    addTagBtn.addEventListener('click', addTag);
    
    // Add tag on Enter key
    tagInput.addEventListener('keypress', function(e) {
        if (e.key === 'Enter') {
            e.preventDefault();
            addTag();
        }
    });

    // Initialize with any existing tags
    if (tagsInput.value) {
        tags = tagsInput.value.split(',').filter(tag => tag.trim() !== '');
        updateTags();
    }
});
