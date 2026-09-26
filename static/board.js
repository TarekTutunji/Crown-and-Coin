// The public projector board.
//
// This page needs no login and can send nothing back to the game: it reads
// /board.json, which only ever carries what the whole room may see (see the
// comment at the top of board.go). It is meant to be opened on the game
// leader's own machine, at localhost:8080/board.html, and thrown on the wall.
//
// A phase does not play by itself. The game leader announces it with the
// banner, then steps through the beats one at a time, so they can stop and talk
// over any of them.

const POLL_MS = 4000;

let board = null;         // What the board is showing
let pending = null;       // A newer phase, waiting for this one to finish
let beats = [];
let at = -1;              // -1 is the phase banner; 0..n-1 is a beat
let shownArmies = {};     // The army strength each realm's card is showing
let shownHealth = {};     // The health each realm's card is showing, so its bar can slide
let lastRevision = null;

const boardGame = document.getElementById('board-game');
const boardRound = document.getElementById('board-round');
const boardPhase = document.getElementById('board-phase');
const boardStatus = document.getElementById('board-status');
const realmsEl = document.getElementById('realms');
const stageEl = document.getElementById('stage');
const progressEl = document.getElementById('progress');
const nextBtn = document.getElementById('next-btn');
const backBtn = document.getElementById('back-btn');
const replayBtn = document.getElementById('replay-btn');
const pendingChip = document.getElementById('pending-chip');
const chronicleSheet = document.getElementById('chronicle-sheet');
const chronicleBody = document.getElementById('chronicle-body');

// Makes text safe to put inside HTML: country and player names are typed in
function esc(value) {
    return String(value).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
}

function titleCase(word) {
    return word ? word.charAt(0).toUpperCase() + word.slice(1) : '';
}

// ---------- Talking to the game ----------

let connected = false;

async function poll() {
    try {
        const response = await fetch('board.json', { cache: 'no-store' });
        if (!response.ok) throw new Error(response.status);
        connected = true;
        receive(await response.json());
    } catch (err) {
        connected = false;
    }
    renderStatus();
}

// The big text names the phase playing out on the stage; the corner says where
// the game itself has got to, which is usually a phase or two ahead
function renderStatus() {
    boardStatus.classList.toggle('live', connected);
    if (!connected) {
        boardStatus.textContent = 'Waiting for the game';
        return;
    }
    boardStatus.textContent = board ? `Round ${board.turn} · ${titleCase(board.phase)} under way` : 'Live';
}

function receive(next) {
    if (next.revision === lastRevision) {
        // The same phase. The standing can still have changed - the game
        // leader may have added a realm or moved a player - so refresh it.
        board = next;
        renderHead();
        renderRealms();
        return;
    }

    // A phase has been resolved. Never cut a replay short: hold the new one
    // back and offer it instead.
    if (beats.length > 0 && at >= 0 && at < beats.length - 1) {
        pending = next;
        pendingChip.textContent = `${titleCase(next.last_phase ? next.last_phase.phase : next.phase)} is ready`;
        pendingChip.classList.remove('hidden');
        return;
    }
    adopt(next);
}

function adopt(next) {
    pending = null;
    pendingChip.classList.add('hidden');
    lastRevision = next.revision;
    // A new game may reuse a realm's name, and the old game's numbers must not
    // bleed into it
    if (board && board.game_name !== next.game_name) {
        shownArmies = {};
        shownHealth = {};
    }
    board = next;
    beats = (next.last_phase && next.last_phase.beats) || [];
    at = -1;
    renderAll();
}

// ---------- Stepping through a phase ----------

function step(by) {
    if (beats.length === 0) return;
    const to = Math.min(beats.length - 1, Math.max(-1, at + by));
    if (to === at) {
        // At the end of a phase, "next" takes the one that was waiting
        if (by > 0 && pending) adopt(pending);
        return;
    }
    at = to;
    renderAll();
}

function replay() {
    if (beats.length === 0) return;
    at = -1;
    renderAll();
}

// ---------- Drawing ----------

function renderAll() {
    renderHead();
    renderRealms();
    renderStage();
    renderControls();
}

function renderHead() {
    if (!board) return;
    boardGame.textContent = board.game_name || '';
    const shown = board.last_phase;
    boardRound.textContent = `Round ${shown ? shown.turn : board.turn}`;
    boardPhase.textContent = shown ? titleCase(shown.phase) : titleCase(board.phase);
    renderStatus();
}

// The beats not yet played are still secret, so a realm's card keeps the old
// number until the beat that changes it has been shown.
function heldHealth(countryID) {
    for (let i = Math.max(at + 1, 0); i < beats.length; i++) {
        const change = beats[i].change;
        if (change && change.label === 'Health' && change.subject === countryID) return change.from;
    }
    return null;
}

function armiesStillSecret() {
    for (let i = Math.max(at + 1, 0); i < beats.length; i++) {
        if (beats[i].armies) return true;
    }
    return false;
}

function renderRealms() {
    if (!board) return;
    const focus = at >= 0 && beats[at] ? (beats[at].focus || []) : [];
    realmsEl.classList.toggle('spotlit', focus.length > 0);

    const secretArmies = armiesStillSecret();
    realmsEl.innerHTML = '';

    (board.realms || []).forEach(realm => {
        const health = heldHealth(realm.country_id);
        const hp = health === null ? realm.hp : health;
        const alive = realm.alive || hp > 0;

        // Until the upkeep beat reveals them, the cards keep the army each
        // realm was last known to have
        let army = realm.army_last_war;
        if (secretArmies && realm.country_id in shownArmies) army = shownArmies[realm.country_id];
        shownArmies[realm.country_id] = army;

        let ruler = 'No ruler';
        if (realm.is_republic) ruler = 'A merchant republic';
        else if (realm.ruler) ruler = esc(realm.ruler);

        let note = '';
        if (!alive) note = '<div class="realm-note">Fallen</div>';
        else if (realm.died_once) note = '<div class="realm-note">Its one revival is spent</div>';

        const merchants = realm.merchants || [];
        const card = document.createElement('div');
        card.className = 'realm';
        if (!alive) card.classList.add('fallen');
        if (focus.includes(realm.country_id)) card.classList.add('lit');
        card.innerHTML = `
            <div class="realm-head">
                <img class="emblem" src="emblems/${realm.is_republic ? 'hall' : 'crown'}.svg" alt="">
                <div>
                    <div class="realm-name">${esc(realm.country_id)}</div>
                    <div class="realm-ruler">${ruler}</div>
                </div>
            </div>
            <div class="realm-bar">
                <div class="realm-bar-fill"></div>
            </div>
            <div class="realm-numbers">
                <span><span class="label">Health</span> ${alive ? hp : 0}/${realm.max_hp}</span>
                <span><span class="label">Army</span> ${army}</span>
            </div>
            <div class="realm-merchants${merchants.length ? '' : ' none'}">
                ${merchants.length ? merchants.map(esc).join(', ') : 'No merchants'}
            </div>
            ${note}
        `;

        // The card is built fresh on every poll, so the bar has to be told
        // where it was before it can slide to where it is now
        const width = value => `${Math.max(0, Math.min(100, Math.round(100 * value / realm.max_hp)))}%`;
        const fill = card.querySelector('.realm-bar-fill');
        const was = realm.country_id in shownHealth ? shownHealth[realm.country_id] : hp;
        fill.style.width = width(was);
        if (was !== hp) {
            requestAnimationFrame(() => requestAnimationFrame(() => { fill.style.width = width(hp); }));
        }
        shownHealth[realm.country_id] = hp;

        realmsEl.appendChild(card);
    });

    const travellers = board.travellers || [];
    if (travellers.length > 0) {
        const line = document.createElement('div');
        line.className = 'travellers';
        line.textContent = 'On the road, arriving next round: ' +
            travellers.map(t => `${t.player_id} to ${t.to_country}`).join(' · ');
        realmsEl.appendChild(line);
    }
}

// What each kind of beat is announced as
const KICKERS = {
    vote: 'Declaration',
    battle: 'Battle',
    damage: 'The cost',
    conquest: 'Conquest',
    collapse: 'Collapse',
    exile: 'Exile',
    revolt: 'Revolt',
    'revolt-failed': 'Revolt',
    government: 'A new order',
    flight: 'Flight',
    arrival: 'Arrival',
    upkeep: 'The armies revealed',
};

function renderStage() {
    stageEl.innerHTML = '';

    if (!board || !board.last_phase || beats.length === 0) {
        stageEl.innerHTML = `
            <div class="stage-idle">
                <p class="stage-idle-line">The board is ready.</p>
                <p class="stage-idle-note">The first phase will play here once the game leader resolves it.</p>
            </div>
        `;
        return;
    }

    if (at < 0) {
        stageEl.appendChild(bannerFor(board.last_phase));
        return;
    }
    stageEl.appendChild(beatFor(beats[at]));
}

function bannerFor(phase) {
    const el = document.createElement('div');
    el.className = 'banner';
    el.innerHTML = `
        <div class="banner-round">Round ${phase.turn}</div>
        <div class="banner-rule"></div>
        <div class="banner-phase">${esc(titleCase(phase.phase))}</div>
        <div class="banner-rule"></div>
        <p class="beat-hint">${beats.length} ${beats.length === 1 ? 'thing' : 'things'} came of it.</p>
    `;
    return el;
}

function beatFor(beat) {
    const el = document.createElement('div');
    el.className = `beat beat-${beat.kind}`;

    const kicker = KICKERS[beat.kind];
    if (kicker) el.insertAdjacentHTML('beforeend', `<div class="beat-kicker">${esc(kicker)}</div>`);

    if (beat.battle) el.appendChild(duelFor(beat.battle));

    el.insertAdjacentHTML('beforeend', `<p class="beat-text">${esc(beat.text)}</p>`);

    if (beat.change) el.appendChild(changeFor(beat.change));
    if (beat.armies) el.appendChild(armyRevealFor(beat.armies));

    return el;
}

// Two crests facing off, with the outcome stamped over them once they have
// charged. Everything is drawn here, so the board costs no downloads.
function duelFor(battle) {
    const el = document.createElement('div');
    el.className = 'duel';

    const side = (id, strength, isRepublic, cls) => `
        <div class="crest ${cls}">
            ${crestSVG(isRepublic)}
            <div class="crest-name">${esc(id)}</div>
            <div class="crest-role">${isRepublic ? 'Republic' : 'Monarchy'}</div>
            <div class="crest-strength">${strength}</div>
            <div class="crest-strength-label">Army</div>
        </div>
    `;

    const outcome = id => {
        if (!battle.winner_id) return '';
        return id === battle.winner_id ? ' won' : ' lost';
    };

    el.innerHTML = `
        ${side(battle.attacker_id, battle.attacker_strength, battle.attacker_republic, 'crest-left' + outcome(battle.attacker_id))}
        ${swordsSVG()}
        ${side(battle.defender_id, battle.defender_strength, battle.defender_republic, 'crest-right' + outcome(battle.defender_id))}
        <div class="stamp${battle.winner_id ? '' : ' stamp-draw'}">${
            battle.winner_id
                ? `${esc(battle.winner_id)} wins &middot; ${battle.damage} damage`
                : 'Stalemate'
        }</div>
    `;
    return el;
}

// A shield with the realm's emblem on it: a crown for a monarchy, the
// merchants' hall for a republic. The same shapes as static/emblems.
function crestSVG(isRepublic) {
    const charge = isRepublic
        ? `<path d="M50 14 L86 40 L86 86 L14 86 L14 40 Z" fill="none" stroke="var(--oxblood)" stroke-width="6" stroke-linejoin="round"></path>
           <path d="M32 86 L32 56 M50 86 L50 56 M68 86 L68 56" stroke="var(--oxblood)" stroke-width="6"></path>`
        : `<path d="M18 70 L13 32 L33 50 L50 22 L67 50 L87 32 L82 70 Z" fill="var(--purple)" stroke="var(--ink)" stroke-width="2.5" stroke-linejoin="round"></path>
           <rect x="18" y="74" width="64" height="9" fill="var(--purple)" stroke="var(--ink)" stroke-width="2.5"></rect>
           <circle cx="13" cy="30" r="4" fill="var(--ink)"></circle><circle cx="50" cy="19" r="4" fill="var(--ink)"></circle><circle cx="87" cy="30" r="4" fill="var(--ink)"></circle>`;

    return `
        <svg viewBox="0 0 200 236" role="img" aria-hidden="true">
            <path d="M16 16 H184 V138 C184 190 142 216 100 230 C58 216 16 190 16 138 Z"
                  fill="var(--card)" stroke="var(--ink)" stroke-width="5" stroke-linejoin="round"></path>
            <path d="M30 30 H170 V136 C170 180 136 202 100 215 C64 202 30 180 30 136 Z"
                  fill="none" stroke="var(--brass)" stroke-width="2.5" stroke-dasharray="5 6"></path>
            <g transform="translate(50 58)">${charge}</g>
        </svg>
    `;
}

function swordsSVG() {
    return `
        <svg class="swords" viewBox="0 0 120 120" role="img" aria-hidden="true">
            <g stroke="var(--ink)" stroke-width="7" stroke-linecap="round">
                <path d="M22 100 L88 30"></path>
                <path d="M98 100 L32 30"></path>
            </g>
            <g stroke="var(--brass-dark)" stroke-width="5" stroke-linecap="round">
                <path d="M74 20 L102 20"></path>
                <path d="M18 20 L46 20"></path>
            </g>
            <circle cx="22" cy="106" r="6" fill="var(--brass)" stroke="var(--ink)" stroke-width="3"></circle>
            <circle cx="98" cy="106" r="6" fill="var(--brass)" stroke="var(--ink)" stroke-width="3"></circle>
        </svg>
    `;
}

// A number before and after, with the bar sliding from one to the other
function changeFor(change) {
    const el = document.createElement('div');
    el.className = 'change';
    el.innerHTML = `
        <div class="change-line">
            <span class="change-from">${change.from}</span>
            <span class="change-arrow">&rarr;</span>
            <span class="change-to">${change.to}</span>
            <span class="change-label">${esc(change.label)}</span>
        </div>
        <div class="change-bar"><div class="change-bar-fill"></div></div>
    `;

    const width = value => change.max > 0 ? `${Math.max(0, Math.min(100, Math.round(100 * value / change.max)))}%` : '0%';
    const fill = el.querySelector('.change-bar-fill');
    fill.style.width = width(change.from);
    requestAnimationFrame(() => requestAnimationFrame(() => { fill.style.width = width(change.to); }));
    return el;
}

function armyRevealFor(armies) {
    const el = document.createElement('div');
    el.className = 'army-reveal';
    el.innerHTML = armies.map((army, i) => `
        <div class="army-card" style="animation-delay: ${i * 90}ms">
            <div class="army-card-name">${esc(army.subject)}</div>
            <div class="army-card-numbers"><span class="was">${army.from}</span> &rarr; <span class="now">${army.to}</span></div>
        </div>
    `).join('');
    return el;
}

function renderControls() {
    const total = beats.length;
    if (total === 0) {
        progressEl.textContent = '';
        nextBtn.disabled = !pending;
        backBtn.disabled = true;
        replayBtn.disabled = true;
        return;
    }
    progressEl.textContent = at < 0 ? `Ready · ${total}` : `${at + 1} / ${total}`;
    nextBtn.disabled = at >= total - 1 && !pending;
    backBtn.disabled = at < 0;
    replayBtn.disabled = false;
}

// ---------- The chronicle ----------

function toggleChronicle() {
    const opening = chronicleSheet.classList.contains('hidden');
    chronicleSheet.classList.toggle('hidden', !opening);
    if (opening) renderChronicle();
}

function renderChronicle() {
    const rounds = (board && board.chronicle) || [];
    if (rounds.length === 0) {
        chronicleBody.innerHTML = '<p class="sheet-empty">Nothing is public yet. The first finished war will unseal the round that led to it.</p>';
        return;
    }
    chronicleBody.innerHTML = rounds.map(round => `
        <div class="sheet-round">
            <div class="sheet-round-title">Round ${round.turn}</div>
            ${(round.phases || []).map(phase => `
                <div class="sheet-phase">${esc(titleCase(phase.phase))}</div>
                ${(phase.lines || []).map(line => `<div class="sheet-line">${esc(line)}</div>`).join('')}
            `).join('')}
        </div>
    `).join('');
}

// ---------- Controls ----------

// A button keeps the keyboard focus after it is clicked, and space would then
// press it again on top of doing its own job, so each button lets go first
function onPress(id, done) {
    document.getElementById(id).addEventListener('click', event => {
        event.currentTarget.blur();
        done();
    });
}

onPress('next-btn', () => step(1));
onPress('back-btn', () => step(-1));
onPress('replay-btn', replay);
onPress('pending-chip', () => { if (pending) adopt(pending); });
onPress('chronicle-btn', toggleChronicle);
onPress('chronicle-close', toggleChronicle);
onPress('full-btn', () => {
    if (document.fullscreenElement) document.exitFullscreen();
    else document.documentElement.requestFullscreen().catch(() => {});
});

// Clicking the stage steps on, so the board works from across the room with a
// presentation remote
stageEl.addEventListener('click', () => step(1));

document.addEventListener('keydown', event => {
    if (!chronicleSheet.classList.contains('hidden')) {
        if (event.key === 'Escape' || event.key.toLowerCase() === 'c') {
            event.preventDefault();
            toggleChronicle();
        }
        return;
    }
    switch (event.key) {
        case ' ':
        case 'Enter':
        case 'ArrowRight':
        case 'PageDown':
            event.preventDefault();
            step(1);
            break;
        case 'ArrowLeft':
        case 'PageUp':
            event.preventDefault();
            step(-1);
            break;
        default:
            switch (event.key.toLowerCase()) {
                case 'r': replay(); break;
                case 'c': toggleChronicle(); break;
                case 'f': document.getElementById('full-btn').click(); break;
            }
    }
});

poll();
setInterval(poll, POLL_MS);
