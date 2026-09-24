- [x] The main should start a webserver serving static content from the `static` folder. In that folder we need an index.html with a js and a css file
- [x] The main needs to have some kind of user management. The user have a name and a secret. When a user opens the website it should ask it for a name and secret and then send it to register on the server
- [x] The main should provide a websocket server where the web clients can connect. The websocket receives messages from the client in json format.

The websocket messages follow that patter:
{"user": "", "secret": "", "payload": {}}

- [x] The server should keep track of the registered users and their secrets. When the server starts it should output the secret of the admin user. The admin user should exist from the beginning.
- [x] If a message from a user has the wrong secret, it should be discarded and a error message printed and returned to the client
- [x] The server should start a game using the engine.
- [x] Check test\scenarios.json to understand the possible messages, create a verification method to check if the "user" is allowed to send this message. The admin user can send any message. The response of the engine should be send back via websocket to the client. Everyone can request get_state

# Now we want to make the interface less technical

## Lets start with the admin window

- [x] The game state should be requested every 5 seconds and rendered in a nice way on the page
- [x] The admin has an interface to create, edit and delete countries. When doing so the server is notified using the appropriate message
- [x] The admin also has an non technical way to re assign players as merchants or monarchs for countries (using the messages)
- [x] The admin can advance the phases, it should send the message to the server

## I observed two bugs that need fixing

- [x] The dropdown for the kingdom closes when a new game state is received. This should only happen if the new game state is actually different.
- [x] Whenever a player or admin sends a message to the server and received the response, it should ask for a game state update

## Player management

- [x] The server should provide a way to get the names of all connected players, don't confuse that with the get_player message of the game engine
- [x] The admin should see the current list of players, whenever a player joins he should get the update message via websocket
- [x] The admin should be only able to create countries with monarchs that actually are in the player pool

## Player interface

- [x] The players should also have a good overview over the game state, think about what can logic/code can be shared with the rendering of the admin page
- [x] The players should have a visualization of their possible actions (get_actions message), this should update regularily, maybe every 5 seconds, so we know when the game state changed

## General

- [x] There should be a logout button

## 

- [x] The drop down for merchant name should be populated by players in admin menu

- [?] There needs to be feedback in the interface which action the player logged in. Maybe we need to change engine to ask for chosen action, have that in the game state

- [x] Separate registration and login

- [x] Invest amount changes on update

- [x] Queue multiple actions
- [x] Reject contradicting actions
- [x] Invest <AMOUNT:0-5> shown
- [x] Can't put money for investing as merchant
- [x] Fix high and low taxation at the same time
- [x] Monarch can't tax merchants but should
- [x] Monarchs start with 10 gold
- [x] Attack action missing?
- [x] Players are not always updated for admin
- [x] Refresh loggs out
- [x] Set fixed admin password
- [x] Write phase and turn
- [x] Login works even though you are not registered
- [x] Players logging in with same name
- [x] History of actions should be shown
- [x] Save history of actions / states

# To test

- [x] show monarch_invest action as something nice and ther should be a name of a player and the up/down buttons don't work. It always gets reset to 0. Zero is valid, others don't show (even no rejection)
- [x] Maybe should reject invest with 0
- [x] Tax merchants with too much money should make error message
- [x] The actions in the player interface should always be sorted alphabetically
- [x] Merchant gold is not calculated correctly after taxation see scenarios.json
- [x] After round, when next round starts with taxation 
    -> Merchants should get 5
    -> Investment should double
- [x] Printing history into file should be phase, state, actions ...
- [x] Try overtaxing a merchant

- [x] Test if high taxation leads to a revolt (-2 hp)

- [x] Try out fleeing
- [x] Merchant revolt works
- [x] No tax on revolt

# Done

- [?] Remove this 'end of turn 1 - taxation' thing from history in frontend

- [x] Remove players does not work
    - Only monarchs and merchants
    - Does not do anything

- [x] change peasants to 5

- [x] Monarch should only be able to attack one country
- [x] Not everybody should have the same peasant revolt
- [x] Increase chance of revolt if none happend

- [x] A country should not die in a attack in one round when multiple countries attack it

- [x] low tax reduces risk to 2/6

- [x] taxation should be 2 gold or 1 gold per peasant
- [x] WHen merhcnats revolt, success depends on gold vs monarch and merchants who remain.    

## new features

- [ ] New game
- [ ] Save game, so can continue after program restart (game state, history)
- () On the main admin page, output the initial seed when starting so i can see it.

- [ ] Add republic voting - Extend actions for merchant republics
- [ ] Republic: Vote on hi/lo taxation and war. Most votes option wins.
    - [ ] Money goes from peasants directly to merchants
    - [ ] Merchants invest into army themselves each 
    - [] Merchants vote on who to declare war on
- [x] Make rules for defeated king from revolt and add king stash.
    - War loss: king takes the whole treasury and becomes a merchant with one of the attackers
    - Revolution: king keeps a flat 5 gold and is exiled to a random *other* living kingdom, the rest of the treasury goes to the revolters


## bugs

- [x] Disable advance phase button for 5 seconds after clicking
    - [/] make it visible

- [/] When killing a country all participants share the peasants and merchants

- [/] remove End of Turn 1 in 'game history'

- [/] Cancel pending actions as player
- (/) Fix high taxation revolt to be random

- [ ] distribution of peasants and merchants
- [ ] What if a country dies from a revolt

# Misc

Unique names for players?

# install

scoop install ngrok