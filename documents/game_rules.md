---

## Essential Variables to Track

To maintain the game state, each country must track the following variables:

- Country Health (HP): Starts at **10**
- Country Army Strength: Starts at **0**
- Country Gold: The royal treasury. Starts at **10**
- Country Peasants: Starts at **5**
- Country Revolt Risk: The chance that a high peasant tax sets off a revolt. Starts at **2 in 6**
- Country state: Is it a monarchy or merchant republic
- Country Belonging: For each Merchant to which country they belong
- Merchant Purse (Per Merchant): Gold the merchant holds openly. Starts at **5**. Their Monarch can see it and tax it.
- Hidden Merchant Gold (Per Merchant): Gold the merchant has hidden. Only the merchant can see it and it can never be taxed.
- Invested Merchant Gold (Per Merchant): Each merchant’s investment that will pay off double at the end of the War phase

---

## The Game Loop: Phase-by-Phase Actions

### Phase 1: Taxation

In this phase, the Monarch generates revenue for the state.

* **Monarch Options:** 
    * **Peasant Tax:** Choose **low** tax (**1 gold** per peasant, no chance of revolt) or **high** tax (**2 gold** per peasant, with a chance of a peasant revolt). A Monarch who does not choose collects the **low** tax.
        * **Revolt risk:** starts at **2 in 6**. Every high tax that does not cause a revolt raises it by one, up to **5 in 6**. A low tax or a revolt resets it to 2 in 6.
        * **Peasant revolt:** the country collects **no gold** from its peasants that round and loses **2 HP**. Revolts are resolved at the end of Phase 1.
    * **Merchant Tax:** Take any amount of gold from a merchant's **purse**, up to everything in it. The gold goes to the Country. Hidden gold is out of reach. The Monarch can see how much each of their merchants has in their purse, but not their hidden or invested gold. Any agreement about how much to tax is made between the players; the game does not enforce it.

### Phase 2: Negotiation

This phase is purely about players talking to each other. No game rules here and nothing needs to be implemented.

### Phase 3: Spending & Investment

This phase determines the country's economic growth and military power for the round.

* **Monarch Options:**
    * **Build Army:** One gold results into one army strength
    * **Invest:** Give gold to merchants. It goes into their purse.
    * **Save:** Keep gold in the royal treasury for later rounds

* **Merchant Options:**
    * **Invest:** Invest gold. This gold **doubles** in value and is paid back at the end of the War phase. The payout lands in the **purse**, so the Monarch can tax it in the next Taxation phase before the merchant gets the chance to hide it.
    * **Hide:** Move any amount of gold from the purse into hidden gold, safe from taxes. Hidden gold stays hidden until the merchant spends it.
    * Hiding is done first. Investing (and, in a republic, paying into the army) then takes gold from the purse first and from hidden gold after that.

### Phase 4: War Phase

The Monarch exercises military power against rivals.

* **Monarch Options:**
    * **Attack:** Choose **one** target country to invade
    * **No Attack:** Stay at home

* **Battles:**
    * Every attack is its own battle. The bigger army wins; an exact tie means nothing happens.
    * All battles use the army strengths from the **start** of the War phase. An army is not used up by fighting: a country attacked by three rivals defends against each of them with its full army.
    * If two countries attack each other, that counts as **two** battles, so the loser takes the damage twice and the winner earns the victory gold twice.

* **Outcomes:**
    * **Victory:** The winner of each battle receives **5 gold**.
    * **Loss:** The loser loses HP equal to the difference in army strength. Damage from several lost battles in the same round is added up. The first time a country dies, it comes back with 1 HP and continues playing.
    * **Annexation:** When a country dies a second time it is permanently eliminated, and the countries that beat it that round take everything it had:
        * A **single conqueror** takes **all** of its merchants and **all** of its peasants.
        * **Several conquerors** split its merchants and peasants as evenly as possible. When they do not divide evenly, the dice decide which conquerors get the leftovers.
    * **Defeated Monarch:** The Monarch of the eliminated country takes the **entire treasury** as personal savings, the treasury is set to **0**, and the Monarch becomes a merchant in one of the conquerors, chosen by the dice when there are several.

* **End of the War phase:**
    * **Maintenance:** After all attacks, all remaining army strength is **halved**.
    * Army strengths become known to all players.
    * Investments pay out **double** into each merchant's purse.
    * Every merchant receives **5 gold** of income into their purse.

### Phase 5: Internal Assessment

The merchants decide if they will continue to support the current regime.

* **Merchant Options:**
    * **Remain:** Stay with the current country and monarch. A merchant who remains throws their gold behind the monarch, helping to put down any revolt that round. A merchant who does not choose anything also stays, but does **not** count as backing the monarch.
    * **Flee:** Move to a different country. They can only take their **savings** (purse and hidden gold), not their investments. Fleeing merchants back neither side.
    * **Revolt:** Overthrow the monarch if the participating merchants have combined more gold than the monarch and the merchants who remained

* **Revolt Mechanics:**
    * **Requirement:** Participating Merchants must collectively have more gold than the Monarch's treasury **plus** the gold of every merchant in that country who chose to **Remain**. A tie goes to the Monarch. Every merchant's purse, hidden and invested gold all count.
    * **Result:** If they meet the gold requirement and choose to act, they succeed automatically. The Monarch is deposed, the country loses **2 HP**, and it becomes a **Merchant Republic**. If that damage kills the country, it collapses instead (see **Death by Revolt** below).
    * **Deposed Monarch:** The deposed Monarch always leaves with a flat **5 gold**, no matter how large or small the treasury was (even an empty treasury). They are **exiled**: they become a merchant in one randomly chosen country among the *other* countries still alive, never in the one that just deposed them. If no other country is left alive, they drop out of the game. Whatever is left of the treasury after the 5 gold is split evenly among the merchants who revolted that round (any remainder goes to the lowest player IDs), leaving the new republic with **0 gold**.
    * **Merchant Republic:** In this state, players vote to decide how the country is run (see below).
    * **Failed Revolt:** If the revolt fails, every merchant who revolted loses **all** of their gold (purse, hidden and invested) to the Monarch's treasury.

---

## Merchant Republics

A monarchy overthrown by a successful revolt becomes a **Merchant Republic** and keeps its name. There is no monarch; the merchants run the country in every phase.

* **Taxation:** Each merchant votes for **low** or **high** peasant tax (same amounts and same revolt risk as under a monarchy). A tie, including nobody voting, means **low** tax. The gold collected is split evenly among **all** merchants of the republic, however they voted (any remainder goes to the lowest player IDs).
* **Negotiation:** Unchanged.
* **Spending:** Each merchant splits their gold however they like between the options below. They just cannot spend more gold than they have.
    * **Invest:** Same as any merchant: the gold doubles and is paid back at the end of the War phase.
    * **Hide:** Keep the gold as savings.
    * **Contribute to the army:** Gold goes into the republic's communal army, **1 gold = 1 army strength**. The communal army is halved after the War phase like any other army.
* **War:** Each merchant votes on which kingdom to attack, or votes for no attack. A target needs a **strict majority of all merchants** in the republic (more than half, counting those who did not vote). Without one, including an exact tie, there is no attack. Only one attack per round. Battles are resolved exactly as for a monarchy, except that the **5 gold** for a victory is split evenly among the republic's merchants instead of going to the treasury.
* **Assessment:** Merchants may only **Remain** or **Flee**. There is no monarch to revolt against. If **every** merchant flees, the republic dies for good.
* **Death:** A republic's deaths count together with any death it had as a monarchy. On its first death overall it is revived at 1 HP and carries on as a republic. On its second it is eliminated: its merchants and peasants are shared out among the conquerors just like when a monarchy is conquered, but each merchant keeps only their **savings** (purse and hidden gold). Their investments and the communal army are lost.

---

## Death by Revolt

A country can also be destroyed from within: by its own peasants during the Taxation phase (monarchy or republic), or by the 2 HP a successful merchant revolt costs. In the second case no republic is founded, and the normal split of the treasury after a revolution does not happen. Nobody conquered the country, so everyone scatters to the surviving countries:

* **The Monarch** (if there is one) escapes with the **entire treasury** as personal savings and becomes a merchant in a randomly chosen surviving country.
* **The Merchants**, including any who just revolted, are shared out as evenly as possible among the surviving countries, with the dice deciding which survivors get any leftovers. Each keeps only their **savings** (purse and hidden gold); investments are lost.
* **The Peasants** are lost. No other country gains them.

---

## The Monarch's Gold

There is no separate personal stash for a Monarch during normal play: the country's treasury **is** the Monarch's money. The split between country and person only happens at the moment the Monarch loses the throne — everything on conquest or collapse, a flat 5 gold on revolution.

---

## End of the Game

The game has no built-in winning condition. It runs until the game leader declares it over.
