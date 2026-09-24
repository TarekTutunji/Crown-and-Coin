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

* **Maintenance:** After all attacks, all remaining army strength is **halved** as a maintenance cost.

### Phase 5: Internal Assessment

The merchants decide if they will continue to support the current regime.

* **Merchant Options:**
    * **Remain:** Stay with the current country and monarch. A merchant who remains throws their gold behind the monarch, helping to put down any revolt that round.
    * **Flee:** Move to a different country. They can only take their **savings**, not their investments. Fleeing merchants back neither side.
    * **Revolt:** Overthrow the monarch if the participating merchants have combined more gold than the monarch and the merchants who remained

* **Revolt Mechanics:**
    * **Requirement:** Participating Merchants must collectively have more gold than the Monarch's treasury **plus** the gold of every merchant in that country who chose to **Remain**. A tie goes to the Monarch.
    * **Result:** If they meet the gold requirement and choose to act, they succeed automatically. The Monarch is killed, the country loses **2 HP**, and it becomes a **Merchant Republic**.
    * **Merchant Republic:** In this state, players vote to decide how the country is run.
    * If the revolt fails all the merchants gold goes to the king