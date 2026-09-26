# Tallage: Coercion and Capital — Game Rules

---

## Players and Setup

- Every player is either a **Monarch** (rules a country) or a **Merchant** (lives in a country). Nobody plays both roles.
- The game leader places the players by hand, spreading them evenly across the countries in teams of 4.

---

## Essential Variables to Track

To maintain the game state, each country must track the following variables:

- Country Health (HP): Starts at **10**. HP never goes up: there is no healing.
- Country Army Strength: Starts at **0**
- Country Gold: The royal treasury. Starts at **10**
- Country Peasants: Starts at **5**. The number of peasants only changes through conquest.
- Country Revolt Risk: The chance that a high peasant tax sets off a revolt. Starts at **2 in 6**
- Country state: Is it a monarchy or merchant republic
- Country Belonging: For each Merchant to which country they belong
- Merchant Purse (Per Merchant): Gold the merchant holds openly. Starts at **5**. Their Monarch can see it and tax it.
- Hidden Merchant Gold (Per Merchant): Gold the merchant has hidden. Starts at **0**. Only the merchant can see it and it can never be taxed.
- Invested Merchant Gold (Per Merchant): Starts at **0**. Each merchant's investment that pays out double at the **start of the next round**

---

## Who Sees What

- **Tax choice:** A Monarch's tax choice is secret during Taxation. Their merchants learn it when Phase 1 ends.
- **War:** Declarations of war are secret and revealed to everyone at the same time.
- **Merchant choices:** Everything a merchant chooses is secret, including Assessment choices and Republic votes.
- **Monarch:** Sees each of their own merchants' purses, but never their hidden gold.
- **Merchants:** See their own country's treasury.
- **Other countries:** Nobody can see inside another country. The only thing you can see about another country is its army size as it stood after the last war (after halving). An army bought in Phase 3 stays hidden until the next war is resolved.

---

## Death, Fallen Monarchs and Moving

- **Death:** A country dies when its HP reaches **0 or below**. This is the same whatever the cause: war, a peasant revolt or a merchant revolt.
- **First death:** The country comes back with exactly **1 HP** and keeps playing.
- **Second death:** The country is eliminated (see **Conquest**, **Merchant Republics** and **Collapse from Within** below).
- **Fallen Monarch:** Any Monarch who loses their throne (by conquest, revolt or collapse) becomes a merchant with **5 gold** in their purse. This gold is newly created by the bank. A fallen Monarch never keeps the treasury.
- **Moving:** Any merchant who moves to another country (fleeing, exile, conquest or collapse) sits out the rest of the current round. They join their new country at the **start of the next round**, before investments pay out and before Taxation. From then on they have full rights there, including voting in a Republic.

---

## The Game Loop: Phase-by-Phase Actions

### Start of Each Round

Before Taxation, in this order:

1. Merchants who moved last round arrive in their new country.
2. Investments pay out into each merchant's **purse**: **double** by default (see **Game Settings**).
3. Taxation begins. Because the payout lands in the purse, the Monarch can tax it right away, before the merchant gets a chance to hide it.

### Phase 1: Taxation

In this phase, the Monarch generates revenue for the state. The peasant tax and the merchant tax are chosen together.

* **Monarch Options:**
    * **Peasant Tax:** Choose **low** tax (**1 gold** per peasant, no chance of revolt) or **high** tax (**2 gold** per peasant, with a chance of a peasant revolt). The gold goes to the treasury. A Monarch who does not choose collects the **low** tax.
    * **Merchant Tax:** Take any amount of gold from any of your merchants' **purses**, up to everything in it. The gold goes to the treasury. Hidden gold is out of reach. Any agreement about how much to tax is made between the players; the game does not enforce it.

* **Peasant Revolts:**
    * **The roll:** Once every tax choice is locked in, a six-sided die is rolled for each country that chose high tax. If the roll is **equal to or lower than** the revolt risk, the peasants revolt.
    * **Revolt risk:** starts at **2 in 6**. Every high tax that does not cause a revolt raises it by one, up to **5 in 6**. A low tax or a revolt resets it to 2 in 6.
    * **Peasant revolt:** the country collects **no gold** from its peasants that round and loses **2 HP**. This can kill the country.

### Phase 2: Negotiation

This phase is purely about players talking to each other. No gold can be passed between players in any way. The only exception is a Monarch's gift in Phase 3.

### Phase 3: Spending & Investment

This phase determines the country's economic growth and military power for the round. Merchants act first, then the Monarch.

* **Merchant Options (resolved first):**
    * **Hide / Unhide:** Move any amount of gold between the purse and hidden gold, in either direction. Hiding happens before anything else.
    * **Invest:** Move gold from the purse into investment. It pays out into the purse at the start of the next round, **double** by default (see **Game Settings**). Gold unhidden this round lands in the purse only after investing, so it can be invested **next round** at the earliest.

* **Monarch Options:**
    * **Gift:** Give treasury gold to a merchant in your own country. It lands in their purse **after** the merchants have acted, so they cannot hide or invest it that round.
    * **Build Army:** One gold buys one army strength.
    * **Save:** Any gold not spent stays in the treasury for later rounds.

### Phase 4: War

The Monarch uses military power against rivals.

* **Monarch Options:**
    * **Attack:** Choose **one** target country to invade
    * **No Attack:** Stay at home

* **Battles:**
    * Every attack is its own battle. The bigger army wins; an exact tie means nothing happens.
    * If two countries attack each other, that counts as **one** battle.
    * All battles use the army strengths from the **start** of the War phase. An army is not used up by fighting: a country attacked by three rivals defends against each of them with its full army.

* **Outcomes:**
    * **Victory:** A winning **attacker** receives **5 gold**, newly created by the bank. A winning **defender** gets nothing.
    * **Loss:** The loser, attacker or defender, loses HP equal to the difference in army strength. Damage from several lost battles in the same round is added up.
    * Deaths are checked only after all battles have been resolved.

* **Conquest:** When a country dies for the second time in war, it is eliminated and the countries that beat it that round share it out:
    * Its **merchants**, **peasants** and **treasury** are split as evenly as possible between them. The dice decide who gets any leftovers.
    * Its fallen **Monarch** becomes a merchant with 5 gold (see **Fallen Monarch**) and is placed at random along with the merchants.
    * Conquered merchants keep their purse and hidden gold. Their investments still pay out double, in their new country, at the start of the next round.

* **End of the War phase, in this order:**
    1. **Maintenance:** All armies are **halved**, rounding down.
    2. Army strengths are revealed to all players.
    3. Every merchant receives **5 gold** of income into their purse.

### Phase 5: Internal Assessment

The merchants decide if they will continue to support the current regime. Each merchant secretly picks one option.

* **Merchant Options:**
    * **Remain:** Stay with the current country and Monarch. A merchant who remains throws their gold behind the Monarch, helping to put down any revolt that round. A merchant who does not choose anything also stays, but does **not** count as backing the Monarch.
    * **Flee:** Move to any other country of their choice, including a Republic. They keep their purse and hidden gold. Their investments are destroyed: removed from the game, not given to anyone. Fleeing merchants back neither side.
    * **Revolt:** Try to overthrow the Monarch.

* **Revolt Mechanics:**
    * **Requirement:** The revolt succeeds if the rebels' combined purse and hidden gold is **more than** the treasury **plus** the purse and hidden gold of every merchant who chose to **Remain**. A tie goes to the Monarch. Invested gold never counts.
    * **Success:** The country loses **2 HP** and becomes a **Merchant Republic**. The Monarch is exiled to a random other country as a merchant with 5 gold (see **Fallen Monarch**). The rebels split the **whole treasury** evenly between them, with the dice deciding who gets any leftovers. If the 2 HP kills the country for the second time, it collapses (see **Collapse from Within**).
    * **Failure:** Each rebel's purse and hidden gold go to the treasury, and each rebel's investments are destroyed. Merchants who chose Remain, or chose nothing, keep their investments.

---

## Merchant Republics

A monarchy overthrown by a successful revolt becomes a **Merchant Republic** and keeps its name. There is no Monarch; the merchants run the country in every phase by secret vote. A Republic can never become a monarchy again.

* **Taxation:** Each merchant votes for **low** or **high** peasant tax. A merchant who does not vote counts as a vote for **low** tax, and a tie means **low** tax, so high tax needs votes from **more than half of all** the republic's merchants. The gold collected is split evenly among **all** merchants of the republic, however they voted, with the dice deciding who gets any leftovers. Revolt risk and peasant revolts work exactly as for a monarchy.
* **Negotiation:** Unchanged.
* **Spending:** Each merchant can invest, hide or unhide, or pay into the republic's shared army (**1 gold = 1 army strength**). Only gold in the **purse** can go to the army: hidden gold cannot, and gold unhidden this round lands in the purse too late, so like investing it can be paid in **next round** at the earliest. The shared army is halved after the War phase like any other army.
* **War:** The merchants vote on one country to attack. An attack needs votes from **more than half of all** the republic's merchants; merchants who do not vote count against it. Battles are resolved exactly as for a monarchy, except that the **5 gold** for a victory is split among the republic's merchants.
* **Assessment:** Merchants may only **Remain** or **Flee**. There is no Monarch to revolt against. If **every** merchant flees, the republic is eliminated and its peasants are destroyed.
* **Death:** Deaths and revolt risk carry over from the monarchy. On its second death in war, the republic is conquered just like a monarchy: its merchants and peasants are shared out among the conquerors, and the merchants' investments pay out double in their new countries at the start of the next round.

---

## Collapse from Within

A country can also be destroyed from within: when its second death comes from a peasant revolt or from the 2 HP a successful merchant revolt costs. Nobody conquered it, so everything scatters:

* **The Monarch** (if there is one) becomes a merchant with 5 gold (see **Fallen Monarch**) in a random surviving country.
* **The Treasury** is destroyed.
* **The Merchants** are spread across the surviving countries. Each keeps only their purse and hidden gold; investments are destroyed.
* **The Peasants** are destroyed. No other country gains them.

If a **successful merchant revolt** causes the collapse, the revolt is settled first: the Monarch is exiled with 5 gold and the rebels split the treasury. Then all the merchants are spread across the surviving countries as above.

---

## The Monarch's Gold

There is no separate personal stash for a Monarch during normal play: the country's treasury **is** the Monarch's money. A Monarch who loses the throne, for any reason, walks away with a flat **5 gold** from the bank and nothing from the treasury.

---

## Game Settings

The game leader can change these at any time from the admin panel. Every player can see the current settings at the top of their screen.

* **Investment return:** How much an investment pays back, as a percentage of the gold put in, from **0%** to **500%**. The default is **200%** (double); 150% pays one and a half times. Fractions of a gold coin are rounded down. A change applies to every investment still waiting to pay out.
* **Open game:** Every player sees every move the way the game leader does. This overrides all the secrecy rules in **Who Sees What**. Switch it off to go back to the limited view.

---

## End of the Game

The game has no built-in winning condition. It runs until the game leader declares it over.

When they do, the projector board turns into a summary of the whole game, in three parts: the **final standings** of the realms, with the armies and treasuries they really hold; **every merchant's gold**, purse, hidden savings and investments alike; and the **story of the game**, round by round — the battles, conquests, revolts, thrones lost and republics made and unmade. Hidden gold can never be taxed and nobody but its owner sees it while the game is being played, but this is where it comes out: it is the one moment all of it becomes public.

Declaring the game over changes nothing in the game itself. The game leader can put the live board back at any time and play on, and resolving another phase brings the live board back by itself.
