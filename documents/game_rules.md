---

## Essential Variables to Track

To maintain the game state, each country must track the following variables:

- Country Health (HP): Starts at **10**
- Country Army Strength: Starts at **0**
- Country Gold: Current liquid capital held by the country
- Country Peasants: Starts with 1
- Country state: Is it a monarchy or merchant republic
- Country Belonging: For each Merchant to which country they belong
- Stored Merchant Gold (Per Merchant): Each merchant’s individual holdings.
- Invested Merchant Gold (Per Merchant): Each merchant’s investment that will pay off double in the next turn

---

## The Game Loop: Phase-by-Phase Actions

### Phase 1: Taxation

In this phase, the Monarch generates revenue for the state.

* **Merchant Action:** Each merchant automatically receives **5 gold** at the start of each turn.

* **Monarch Options:** 
    * **Peasant Tax:** Choose to collect **5 gold** (no chance of revolt) or **10 gold** per peasant (2/6 chance of revolt resulting in -2HP; the revolt is resolved only at the end of Phase 1)
    * **Merchant Tax:** Collect an agreed-upon or mandated amount of gold from the merchants that goes to the Country

### Phase 2: Negotiation

This phase is purely about players talking to each other. No game rules here and nothing needs to be implemented.

### Phase 3: Spending & Investment

This phase determines the country's economic growth and military power for the round.

* **Monarch Options:**
    * **Build Army:** One gold results into one army strength
    * **Invest:** Give gold to merchants
    * **Save:** Keep gold in the royal treasury for later rounds

* **Merchant Options:**
    * **Invest:** Invest gold. This gold **doubles** in value and is payed back in next round before Phase 1.
    * **Hide:** Put gold into personal savings

### Phase 4: War Phase

The Monarch exercises military power against rivals.

* **Monarch Options:**
    * **Attack:** Choose a target country to invade

* **Outcomes:**
    * **Victory:** The winner receives **5 gold**.
    * **Loss:** The loser loses HP the difference of army strength. The first time you die, you get 1 HP and continue playing.
    * **Annexation:** If a country is defeated, the winner takes their merchants and the winning country gets one peasant.
    * **Defeated Monarch:** When a country dies a second time it is permanently eliminated. Its Monarch takes the **entire treasury** as personal savings, the treasury is set to **0**, and the Monarch becomes a merchant in one of that round's attackers, chosen at random when several countries attacked together.

* **Maintenance:** After all attacks, all remaining army strength is **halved** as a maintenance cost.

### Phase 5: Internal Assessment

The merchants decide if they will continue to support the current regime.

* **Merchant Options:**
    * **Remain:** Stay with the current country and monarch. A merchant who remains throws their gold behind the monarch, helping to put down any revolt that round.
    * **Flee:** Move to a different country. They can only take their **savings**, not their investments. Fleeing merchants back neither side.
    * **Revolt:** Overthrow the monarch if the participating merchants have combined more gold than the monarch and the merchants who remained

* **Revolt Mechanics:**
    * **Requirement:** Participating Merchants must collectively have more gold than the Monarch's treasury **plus** the gold of every merchant in that country who chose to **Remain**. A tie goes to the Monarch.
    * **Result:** If they meet the gold requirement and choose to act, they succeed automatically. The Monarch is deposed, the country loses **2 HP**, and it becomes a **Merchant Republic**.
    * **Deposed Monarch:** The deposed Monarch keeps a flat **5 gold** no matter how large the treasury was, and is **exiled** — they become a merchant in one randomly chosen country among the *other* countries still alive, never in the one that just deposed them. If no other country is left alive, they drop out of the game. Whatever is left of the treasury is split evenly among the merchants who revolted that round (any remainder goes to the lowest player IDs), leaving the new republic with **0 gold**.
    * **Merchant Republic:** In this state, players vote to decide how the country is run (see below).
    * If the revolt fails all the merchants gold goes to the king

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
* **Death:** A republic's deaths count together with any death it had as a monarchy. On its first death overall it is revived at 1 HP and carries on as a republic. On its second it is eliminated: its merchants are shared out among the attackers just like when a monarchy is conquered, but each keeps only their **hidden** gold. Their investments and the communal army are lost.

---

## Death by Peasant Revolt

A country, monarchy or republic, can also be destroyed by its own peasants during the Taxation phase. Nobody conquered it, so everyone scatters to the surviving countries:

* **The Monarch** (if there is one) escapes with the **entire treasury** as personal savings and becomes a merchant in a randomly chosen surviving country.
* **The Merchants** are shared out evenly among the surviving countries. Each keeps only their **hidden** gold; investments are lost.

---

## The Monarch's Gold

There is no separate personal stash for a Monarch during normal play: the country's treasury **is** the Monarch's money. The split between country and person only happens at the moment the Monarch loses the throne — everything on conquest, a flat 5 gold on revolution.