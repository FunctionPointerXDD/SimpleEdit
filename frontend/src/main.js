import './style.css';
import {createWizard} from './wizard.js';
import {loadStep} from './steps/load.js';
import {trimStep} from './steps/trim.js';
import {maskStep} from './steps/mask.js';
import {saveStep} from './steps/save.js';

// 글자 칸 밖의 오른쪽 클릭 메뉴(새로고침 등)를 막는다.
function blockContextMenu(e) {
    const t = e.target;
    if (!(t instanceof HTMLInputElement && t.type === 'text')) e.preventDefault();
}

document.addEventListener('contextmenu', blockContextMenu);
createWizard(document.getElementById('app'), [loadStep, trimStep, maskStep, saveStep]);
