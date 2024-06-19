const TagsJoiner = '#get-tags';
const allTags = document.querySelector(TagsJoiner);

const input = document.querySelector('#hashtags');
const container = document.querySelector('.tag-container');
let hashtagArray = [];

input.addEventListener('keyup', (event) => {
    if (event.which === 32 && input.value.trim().length > 0) {
        const newtags = input.value;
        newtags.split(' ').forEach(element => {
            if (isValid(element)) {
                const text = document.createTextNode(element);
                const p = document.createElement('p');
                p.classList.add('tag');
                p.appendChild(text);
                container.appendChild(p);
                hashtagArray.push(element);
            }
        });

        input.value = '';
        allTags.value = hashtagArray.join(' ');

        document.querySelectorAll('.tag').forEach(tag => {
            tag.addEventListener('click', () => {
                container.removeChild(tag);
                const name = tag.innerHTML;
                hashtagArray = hashtagArray.filter(value => value !== name);
                allTags.value = hashtagArray.join(' ');
            });
        });
    }
});

function isValid(tag) {
    if (tag === "" || hashtagArray.includes(tag)) {
        return false;
    }
    if (tag.length > 30 || hashtagArray.length > 50) {
        return false;
    }
    return true;
}
