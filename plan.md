The main feature of this application that it is self hostable, and that it need to give access only to their google calendar and their notes which will be the main source of how agent is going to behave. 

1. The application must be in web, where users can see their chat, their goals how close they to their goals and maybe their tasks through day, plus it must show some gamification, so some cells for example which are going to be filled by the end of the day. This application must self hostable so they can host one binary on their server have authentication and access this interface from any device. Application must know users notes folder and where they can write dayle notes.   
2. The agent must have unique for user knowledge base, this base might be also a markdown file which also will be stored inside of notes folder, there we will store information about user it is essentialy memory, there we can also place some information about which tasks need more time for user, so over time agent will be more and more user adopted.
3. The agent must have some patterns system as fabric does:
    Patterns example:
    1. Analyse my day
    Based on event in user calendar, provide a insightfull feedback about users day, give hints what to update in their day and how to be in track with their goals. Update your knowledge base, based on how the tasks were done.
    2.  Track my day
    See how is day is going on so far, check if we are on track with tasks, if not ask user if they need to shift their tasks by some anouts of minutes
    3. Plan my day
    Based on last 10 dayle notes and our knowledge base and todays note tommorow section plan next day, if user asks plan more strictly where each minute will be planned or more flexible where it might be a litle more time on some tasks. 
4. Agent when planning for tommorow must take the input either from todays note, tommorow section, which will be mentioning tasks which we need to make.
5. If user doesn't have any notes, we might propose to write today note, and asking to fill tommorow section so we know what user might want to plan.
Prompt examples:
1. I dont like current task, remove it from calendar.
2. I will have training in 1.5 hour update my plan accordingly


Self host is becomes harder because to make it self host we would have to create our own google cloud app and then go into oauth and get client_id and client_secret and after that we need to add allowed scopes for the app and only after that we would be able to login after we allow for a tester user because it takes 3-7 days for google to check us.

