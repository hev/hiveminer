#!/usr/bin/env python3
"""
Score and rank Reddit posts for relevance to:
"Alpine Christmas markets combined with nearby skiing in German-speaking Alpine towns"

Reads persisted tool output JSON files + manually specified posts,
deduplicates, scores, ranks, and writes top 60 to discovery_results.json.
"""

import json
import os
from pathlib import Path

TOOL_RESULTS_DIR = "/Users/hev/.claude/projects/-Users-hev-workspace-hev-hiveminer/acf41aff-3382-40b6-8767-41ba0ca0d062/tool-results/"
OUTPUT_PATH = "/Users/hev/workspace/hev/hiveminer/output/christmas-market-skiing-alps-20260216-062028/discovery_results.json"

MANUAL_POSTS = [
    {"id": "1k7ygd", "title": "Ideas for Skiing and Christmas in Europe", "score": 2, "num_comments": 2, "subreddit": "skiing", "permalink": "/r/skiing/comments/1k7ygd/ideas_for_skiing_and_christmas_in_europe/"},
    {"id": "1lob46t", "title": "What\u2019s the best skiing destination in Austria in Late November or early December that will also be fun for non-enthusiasts?", "score": 12, "num_comments": 49, "subreddit": "skiing", "permalink": "/r/skiing/comments/1lob46t/whats_the_best_skiing_destination_in_austria_in/"},
    {"id": "1m9w2jf", "title": "St. Anton or Lech or split the time?", "score": 7, "num_comments": 24, "subreddit": "skiing", "permalink": "/r/skiing/comments/1m9w2jf/st_anton_or_lech_or_split_the_time/"},
    {"id": "11dkk1c", "title": "Skiing Recommendations in Europe?", "score": 3, "num_comments": 19, "subreddit": "skiing", "permalink": "/r/skiing/comments/11dkk1c/skiing_recommendations_in_europe/"},
    {"id": "1k59mhs", "title": "Has anyone gone skiing in Europe (Chamonix) for Christmas?", "score": 6, "num_comments": 77, "subreddit": "skiing", "permalink": "/r/skiing/comments/1k59mhs/has_anyone_gone_skiing_in_europe_chamonix_for/"},
    {"id": "1o7kbsm", "title": "Ski Resort for Christmas (Europe)", "score": 4, "num_comments": 22, "subreddit": "skiing", "permalink": "/r/skiing/comments/1o7kbsm/ski_resort_for_christmas_europe/"},
    {"id": "1n7hxig", "title": "Favourite Alps resorts for Christmas - any and all suggestions welcome!", "score": 5, "num_comments": 17, "subreddit": "skiing", "permalink": "/r/skiing/comments/1n7hxig/favourite_alps_resorts_for_christmas_any_and_all/"},
    {"id": "1ji62it", "title": "Thinking about skiing the Italian/French alps next Christmas", "score": 2, "num_comments": 20, "subreddit": "skiing", "permalink": "/r/skiing/comments/1ji62it/thinking_about_skiing_the_italianfrench_alps_next/"},
    {"id": "1mqlxpb", "title": "Skiing (maybe cross country) near Munich in late December", "score": 1, "num_comments": 1, "subreddit": "skiing", "permalink": "/r/skiing/comments/1mqlxpb/skiing_maybe_cross_country_near_munich_in_late/"},
    {"id": "1io2rxs", "title": "Americans in the Alps", "score": 528, "num_comments": 478, "subreddit": "skiing", "permalink": "/r/skiing/comments/1io2rxs/americans_in_the_alps/"},
    {"id": "r0id9i", "title": "I want to do a solo ski trip to the Alps next year", "score": 78, "num_comments": 73, "subreddit": "skiing", "permalink": "/r/skiing/comments/r0id9i/i_want_to_do_a_solo_ski_trip_to_the_alps_next/"},
    {"id": "qict96", "title": "What are the most underrated skiing resorts in the Alps?", "score": 40, "num_comments": 60, "subreddit": "skiing", "permalink": "/r/skiing/comments/qict96/what_are_the_most_underrated_skiing_resorts_in/"},
    {"id": "6ph11o", "title": "Help Plan a Trip", "score": 2, "num_comments": 9, "subreddit": "skiing", "permalink": "/r/skiing/comments/6ph11o/help_plan_a_trip/"},
    {"id": "186xcz0", "title": "Do the German Christmas markets take Apple Pay/cc?", "score": 26, "num_comments": 47, "subreddit": "travel", "permalink": "/r/travel/comments/186xcz0/do_the_german_christmas_markets_take_apple_paycc/"},
    {"id": "qnxja9", "title": "Bavaria Two Week Itinerary", "score": 3, "num_comments": 44, "subreddit": "travel", "permalink": "/r/travel/comments/qnxja9/bavaria_two_week_itinerary/"},
    {"id": "1ooitw5", "title": "Reccomendations on where to go skiing during the Christmas holidays that will have seasonal festivities and decent snow", "score": 2, "num_comments": 15, "subreddit": "skithealps", "permalink": "/r/skithealps/comments/1ooitw5/reccomendations_on_where_to_go_skiing_during_the/"},
    {"id": "1q39ibz", "title": "Ski hotel for Christmas or mid January?", "score": 2, "num_comments": 6, "subreddit": "skithealps", "permalink": "/r/skithealps/comments/1q39ibz/ski_hotel_for_christmas_or_mid_january/"},
    {"id": "1pkgpxw", "title": "Best ski options this Christmas?", "score": 5, "num_comments": 8, "subreddit": "skithealps", "permalink": "/r/skithealps/comments/1pkgpxw/best_ski_options_this_christmas/"},
    {"id": "1pob4yb", "title": "Ischgl vs Kitzb\u00fchel vs Meg\u00e8ve for long cruisers + views", "score": 5, "num_comments": 25, "subreddit": "skithealps", "permalink": "/r/skithealps/comments/1pob4yb/ischgl_vs_kitzb\u00fchel_vs_meg\u00e8ve_for_long_cruisers/"},
    {"id": "1mckyee", "title": "Any resort recommendations for Christmas/January and March?", "score": 6, "num_comments": 9, "subreddit": "skithealps", "permalink": "/r/skithealps/comments/1mckyee/any_resort_recommendations_for_christmasjanuary/"},
    {"id": "1pj6ono", "title": "Salzburg Skiing", "score": 2, "num_comments": 5, "subreddit": "skithealps", "permalink": "/r/skithealps/comments/1pj6ono/salzburg_skiing/"},
    {"id": "1nyz9n1", "title": "Early December Ski Resort recommendations", "score": 3, "num_comments": 14, "subreddit": "skithealps", "permalink": "/r/skithealps/comments/1nyz9n1/early_december_ski_resort_recommendations/"},
    {"id": "1fhkgmz", "title": "Early December Skiing", "score": 5, "num_comments": 6, "subreddit": "skithealps", "permalink": "/r/skithealps/comments/1fhkgmz/early_december_skiing/"},
    {"id": "1ntovrk", "title": "Ischgl vs Val Thorens", "score": 6, "num_comments": 38, "subreddit": "skithealps", "permalink": "/r/skithealps/comments/1ntovrk/ischgl_vs_val_thorens/"},
    {"id": "1kgjkkz", "title": "Does this resort exist?", "score": 5, "num_comments": 35, "subreddit": "skithealps", "permalink": "/r/skithealps/comments/1kgjkkz/does_this_resort_exist/"},
    {"id": "1p6gvp1", "title": "That\u2019s it, I wanna ski the alps. Looking for suggestions", "score": 14, "num_comments": 50, "subreddit": "skithealps", "permalink": "/r/skithealps/comments/1p6gvp1/thats_it_i_wanna_ski_the_alps_looking_for/"},
    {"id": "1pisg3t", "title": "Looking for input", "score": 3, "num_comments": 2, "subreddit": "skithealps", "permalink": "/r/skithealps/comments/1pisg3t/looking_for_input/"},
    {"id": "17cmvwi", "title": "Ski Resorts on Christmas - I\u2019m confused", "score": 0, "num_comments": 20, "subreddit": "Austria", "permalink": "/r/Austria/comments/17cmvwi/ski_resorts_on_christmas_im_confused/"},
    {"id": "5j86sx", "title": "Best Christmas-time Skiing Near Salzburg?", "score": 4, "num_comments": 6, "subreddit": "Austria", "permalink": "/r/Austria/comments/5j86sx/best_christmastime_skiing_near_salzburg_beste_ski/"},
    {"id": "ar9kxv", "title": "Austrian Winter Honeymoon Suggestions", "score": 3, "num_comments": 16, "subreddit": "Austria", "permalink": "/r/Austria/comments/ar9kxv/austrian_winter_honeymoon_suggestions/"},
    {"id": "173w35h", "title": "Going to Innsbruck for Christmas and New Year!", "score": 2, "num_comments": 16, "subreddit": "Innsbruck", "permalink": "/r/Innsbruck/comments/173w35h/going_to_innsbruck_for_christmas_and_new_year/"},
    {"id": "1i0yv7t", "title": "5 Nights in Innsbruck over Christmas", "score": 0, "num_comments": 6, "subreddit": "Innsbruck", "permalink": "/r/Innsbruck/comments/1i0yv7t/5_nights_in_innsbruck_over_christmas/"},
    {"id": "1olljr5", "title": "Innsbruck 19-23 Dec: Last-minute Christmas Market check + Igls advice needed!", "score": 1, "num_comments": 2, "subreddit": "Innsbruck", "permalink": "/r/Innsbruck/comments/1olljr5/innsbruck_1923_dec_lastminute_christmas_market/"},
    {"id": "1p9t6pc", "title": "How do I visit the Nordkette?", "score": 4, "num_comments": 11, "subreddit": "Innsbruck", "permalink": "/r/Innsbruck/comments/1p9t6pc/how_do_i_visit_the_nordkette/"},
    {"id": "1mo05o0", "title": "Stubaier Gletscher holiday December 24-25", "score": 0, "num_comments": 5, "subreddit": "Innsbruck", "permalink": "/r/Innsbruck/comments/1mo05o0/stubaier_gletscher_holiday_december_2425/"},
    {"id": "1052v5l", "title": "How\u2019s the snow looking at the resorts near Innsbruck?", "score": 3, "num_comments": 25, "subreddit": "Innsbruck", "permalink": "/r/Innsbruck/comments/1052v5l/hows_the_snow_looking_at_the_resorts_near/"},
    {"id": "1qnujyv", "title": "Christmas Luxe Ski Vacation", "score": 2, "num_comments": 7, "subreddit": "chubbytravel", "permalink": "/r/chubbytravel/comments/1qnujyv/christmas_luxe_ski_vacation/"},
    {"id": "1q8tccf", "title": "Family Ski Trip?", "score": 1, "num_comments": 9, "subreddit": "chubbytravel", "permalink": "/r/chubbytravel/comments/1q8tccf/family_ski_trip/"},
]

CHRISTMAS_MARKET_KW = [
    "christmas market", "xmas market", "weihnachtsmarkt", "christkindlmarkt",
    "christkindlesmarkt", "advent market", "adventmarkt", "christmas village",
    "holiday market", "festive market", "winter market", "gluehwein",
    "mulled wine", "seasonal festivit", "christmas bazaar",
]

SKIING_KW = [
    "ski", "skiing", "ski resort", "ski trip", "ski holiday", "ski vacation",
    "slopes", "piste", "apres-ski", "apres ski", "snowboard",
    "lift pass", "ski pass", "cross country ski", "cross-country ski",
]

GERMAN_SPEAKING_ALPINE_KW = [
    "austria", "austrian", "tirol", "tyrol", "innsbruck", "salzburg",
    "kitzb\u00fchel", "kitzbuhel", "kitzbuehel", "zell am see", "st. anton",
    "st anton", "lech", "mayrhofen", "ischgl", "bad gastein",
    "saalbach", "hinterglemm", "stubai",
    "bavaria", "bavarian", "garmisch", "partenkirchen", "zugspitze",
    "munich", "berchtesgaden", "oberstdorf",
    "south tyrol", "alto adige", "bolzano", "merano", "meran",
    "bressanone", "brixen", "vipiteno", "sterzing", "dolomites",
    "switzerland", "swiss", "davos", "st. moritz",
    "st moritz", "grindelwald", "zermatt", "verbier", "jungfrau",
    "engadin",
    "alps", "alpine",
]

OFF_TOPIC_KW = [
    "colorado", "utah", "vermont", "montana", "wyoming", "tahoe",
    "whistler", "japan", "niseko", "jackson hole", "aspen", "vail",
    "park city", "mammoth", "big sky", "steamboat", "telluride",
    "breckenridge", "keystone",
]

PERFECT_TOWNS = [
    "innsbruck", "salzburg", "kitzb\u00fchel", "kitzbuhel", "garmisch",
    "bolzano", "merano", "berchtesgaden", "hall in tirol",
    "bressanone", "brixen", "vipiteno", "sterzing", "seefeld",
    "kufstein", "bad reichenhall", "zell am see",
]


def load_persisted_posts():
    posts = []
    tool_dir = Path(TOOL_RESULTS_DIR)
    for fpath in sorted(tool_dir.glob("*.txt")):
        try:
            with open(fpath) as f:
                data = json.load(f)
            if isinstance(data, list):
                for item in data:
                    if isinstance(item, dict) and "id" in item:
                        posts.append(item)
        except (json.JSONDecodeError, KeyError):
            continue
    return posts


def deduplicate(posts):
    seen = {}
    for p in posts:
        pid = p["id"]
        if pid not in seen:
            seen[pid] = p
        else:
            existing = seen[pid]
            existing_richness = len(existing.keys()) + len(existing.get("selftext", "") or "")
            new_richness = len(p.keys()) + len(p.get("selftext", "") or "")
            if new_richness > existing_richness:
                seen[pid] = p
    return list(seen.values())


def kw_in(keywords, text):
    return sum(1 for kw in keywords if kw in text)


def any_in(keywords, text):
    return any(kw in text for kw in keywords)


def score_post(post):
    title = (post.get("title") or "").lower()
    selftext = (post.get("selftext") or "").lower()
    subreddit = (post.get("subreddit") or "").lower()
    num_comments = post.get("num_comments", 0) or 0
    reddit_score = post.get("score", 0) or 0
    combined = title + " " + selftext

    score = 0.0
    reasons = []

    # ── Title-based signals (high weight - titles are very high-signal) ──
    title_christmas_market = kw_in(CHRISTMAS_MARKET_KW, title) > 0
    title_christmas = title_christmas_market or any_in(["christmas", "xmas", "weihnacht"], title)
    title_skiing = kw_in(SKIING_KW, title) > 0
    title_alpine = kw_in(GERMAN_SPEAKING_ALPINE_KW, title) > 0
    title_towns = kw_in(PERFECT_TOWNS, title)

    # Title has BOTH christmas + skiing -> strongest possible signal
    if title_christmas and title_skiing:
        score += 45
        reasons.append("title combines christmas + skiing (+45)")
    elif title_christmas:
        score += 12
        reasons.append("title mentions christmas (+12)")
    if title_skiing and not (title_christmas and title_skiing):
        score += 10
        reasons.append("title mentions skiing (+10)")
    if title_alpine:
        score += 10
        reasons.append("title mentions Alpine region (+10)")
    if title_towns > 0:
        bonus = min(title_towns * 8, 20)
        score += bonus
        reasons.append(f"title mentions {title_towns} specific Alpine town(s) (+{bonus})")

    # ── Body-based signals (capped to prevent long-text domination) ──────
    body_christmas_market = kw_in(CHRISTMAS_MARKET_KW, selftext)
    body_christmas = body_christmas_market > 0 or any_in(["christmas", "xmas", "weihnacht"], selftext)
    body_skiing = kw_in(SKIING_KW, selftext)
    body_alpine = kw_in(GERMAN_SPEAKING_ALPINE_KW, selftext)
    body_towns = kw_in(PERFECT_TOWNS, selftext)

    if body_christmas_market > 0:
        bonus = min(body_christmas_market * 5, 15)
        score += bonus
        reasons.append(f"body christmas-market mentions (+{bonus})")
    elif body_christmas:
        score += 4
        reasons.append("body christmas reference (+4)")

    if any_in(["december", "dezember", "advent"], combined):
        score += 3
        reasons.append("december/advent reference (+3)")

    if body_skiing > 0:
        bonus = min(body_skiing * 4, 12)
        score += bonus
        reasons.append(f"body skiing mentions (+{bonus})")

    # Body has BOTH christmas + skiing (additional combo bonus)
    has_christmas = title_christmas or body_christmas or body_christmas_market > 0
    has_skiing = title_skiing or body_skiing > 0
    if has_christmas and has_skiing and not (title_christmas and title_skiing):
        score += 20
        reasons.append("body-level christmas + skiing combo (+20)")

    if body_alpine > 0:
        bonus = min(body_alpine * 3, 15)
        score += bonus
        reasons.append(f"body Alpine references (+{bonus})")

    if body_towns > 0:
        bonus = min(body_towns * 4, 15)
        score += bonus
        reasons.append(f"body mentions {body_towns} specific town(s) (+{bonus})")

    # ── Subreddit bonus ──────────────────────────────────────────────────
    # Specialized Alpine/ski/regional subs get a strong bonus because
    # their posts are inherently about the right topic even with short text
    sub_bonuses = {
        "skithealps": 15, "austria": 12, "innsbruck": 15,
        "europetravel": 5, "germany": 8, "fattravel": 4,
        "chubbytravel": 4, "travel": 2, "solotravel": 2, "skiing": 5,
    }
    sub_bonus = sub_bonuses.get(subreddit, 0)
    if sub_bonus:
        score += sub_bonus
        reasons.append(f"relevant subreddit r/{subreddit} (+{sub_bonus})")

    # ── Comment count (extractable data) ─────────────────────────────────
    if num_comments >= 100:
        score += 15
        reasons.append(f"high comment count {num_comments} (+15)")
    elif num_comments >= 40:
        score += 10
        reasons.append(f"good comment count {num_comments} (+10)")
    elif num_comments >= 15:
        score += 6
        reasons.append(f"moderate comments {num_comments} (+6)")
    elif num_comments >= 5:
        score += 3
        reasons.append(f"some comments {num_comments} (+3)")
    elif num_comments <= 1:
        score -= 5
        reasons.append(f"very few comments {num_comments} (-5)")

    # ── Recommendation/discussion pattern ────────────────────────────────
    rec_patterns = [
        "recommend", "suggestion", "itinerary", "trip report", "advice",
        "help plan", "where to", "best place", "which resort", "looking for",
        "any tips", "ideas for", "options for", "what to do",
    ]
    if any(p in combined for p in rec_patterns):
        score += 5
        reasons.append("recommendation/discussion pattern (+5)")

    # ── Off-topic penalties ──────────────────────────────────────────────
    off_topic_hits = kw_in(OFF_TOPIC_KW, combined)
    if off_topic_hits > 0:
        penalty = min(off_topic_hits * 10, 40)
        score -= penalty
        reasons.append(f"off-topic location mentions (-{penalty})")

    if "apple pay" in combined or "what to wear" in combined or "what to buy" in combined:
        score -= 20
        reasons.append("logistics-only post (-20)")

    if not has_christmas and not has_skiing and not title_alpine and body_alpine == 0:
        score -= 20
        reasons.append("no relevant topic signals (-20)")

    # ── Reddit score bonus ───────────────────────────────────────────────
    if reddit_score >= 100:
        score += 5
        reasons.append(f"high reddit score {reddit_score} (+5)")
    elif reddit_score >= 20:
        score += 2
        reasons.append(f"decent reddit score {reddit_score} (+2)")

    return score, "; ".join(reasons)


def generate_reason(post):
    title = post.get("title", "")
    subreddit = post.get("subreddit", "")
    num_comments = post.get("num_comments", 0) or 0
    combined = (title + " " + (post.get("selftext") or "")).lower()

    has_christmas_market = any_in(CHRISTMAS_MARKET_KW, combined)
    has_christmas = has_christmas_market or "christmas" in combined
    has_skiing = any_in(SKIING_KW, combined)
    has_alpine = any_in(GERMAN_SPEAKING_ALPINE_KW, combined)

    parts = []

    if has_christmas_market and has_skiing:
        parts.append("Directly discusses combining Christmas markets with skiing in the Alps")
    elif has_christmas and has_skiing:
        parts.append("Discusses skiing during the Christmas/holiday season")
    elif has_christmas_market:
        parts.append("Focuses on Christmas markets in Alpine region")
    elif has_christmas:
        parts.append("Discusses Christmas/holiday season travel in Alpine area")
    elif has_skiing and has_alpine:
        parts.append("Discusses Alpine skiing with potential Christmas-season context")
    elif has_skiing:
        parts.append("Discusses skiing that may include Alpine resort recommendations")
    elif has_alpine:
        parts.append("Discusses travel in German-speaking Alpine region")

    locations = []
    loc_map = {
        "innsbruck": "Innsbruck", "salzburg": "Salzburg",
        "kitzb\u00fchel": "Kitzb\u00fchel", "kitzbuhel": "Kitzb\u00fchel",
        "garmisch": "Garmisch", "austria": "Austria",
        "tirol": "Tyrol", "tyrol": "Tyrol",
        "bolzano": "Bolzano", "merano": "Merano",
        "bavaria": "Bavaria", "munich": "Munich",
        "ischgl": "Ischgl",
        "st. anton": "St. Anton", "st anton": "St. Anton",
        "lech": "Lech", "stubai": "Stubai",
        "dolomites": "Dolomites", "south tyrol": "South Tyrol",
        "zell am see": "Zell am See", "saalbach": "Saalbach",
        "davos": "Davos", "st. moritz": "St. Moritz",
        "st moritz": "St. Moritz", "switzerland": "Switzerland",
    }
    for kw, name in loc_map.items():
        if kw in combined and name not in locations:
            locations.append(name)
    if locations:
        parts.append(f"mentions {', '.join(locations[:5])}")

    if num_comments >= 40:
        parts.append(f"with {num_comments} comments likely containing specific resort and town recommendations")
    elif num_comments >= 15:
        parts.append(f"with {num_comments} comments likely containing useful destination details")
    elif num_comments >= 5:
        parts.append(f"with {num_comments} comments that may contain relevant suggestions")

    if not parts:
        parts.append(f"Post in r/{subreddit} that may contain tangentially relevant discussion")

    return ". ".join(parts) + "."


def main():
    print("Loading persisted tool results...")
    persisted = load_persisted_posts()
    print(f"  Loaded {len(persisted)} posts from tool results")

    all_posts = persisted + MANUAL_POSTS
    print(f"  Added {len(MANUAL_POSTS)} manual posts, total before dedup: {len(all_posts)}")

    unique_posts = deduplicate(all_posts)
    print(f"  After deduplication: {len(unique_posts)} unique posts")

    scored = []
    for post in unique_posts:
        numeric_score, rationale = score_post(post)
        reason = generate_reason(post)
        scored.append((numeric_score, rationale, reason, post))

    scored.sort(key=lambda x: x[0], reverse=True)

    print("\nScore distribution:")
    for threshold in [100, 80, 60, 40, 20, 0]:
        count = sum(1 for s, _, _, _ in scored if s >= threshold)
        print(f"  >= {threshold:3d}: {count} posts")

    top_60 = scored[:60]
    print(f"\nSelected top {len(top_60)} posts")
    print(f"  Score range: {top_60[-1][0]:.0f} to {top_60[0][0]:.0f}")

    output_posts = []
    for numeric_score, rationale, reason, post in top_60:
        output_posts.append({
            "id": post["id"],
            "title": post.get("title", ""),
            "permalink": post.get("permalink", ""),
            "subreddit": post.get("subreddit", ""),
            "score": post.get("score", 0),
            "num_comments": post.get("num_comments", 0),
            "reason": reason,
        })

    search_log = [
        {"query": "christmas market skiing", "subreddit": "Europetravel", "results": 25},
        {"query": "christmas market skiing", "subreddit": "travel", "results": 25},
        {"query": "christmas market skiing", "subreddit": "solotravel", "results": 25},
        {"query": "christmas market skiing", "subreddit": "skiing", "results": 25},
        {"query": "christmas market skiing", "subreddit": "FATTravel", "results": 25},
        {"query": "christmas market skiing", "subreddit": "chubbytravel", "results": 25},
        {"query": "christmas market skiing", "subreddit": "skithealps", "results": 20},
        {"query": "christmas market skiing", "subreddit": "germany", "results": 25},
        {"query": "christmas market skiing", "subreddit": "Austria", "results": 25},
        {"query": "christmas market skiing", "subreddit": "Innsbruck", "results": 20},
        {"query": "christmas market near ski resort alps", "subreddit": "Europetravel", "results": 25},
        {"query": "Innsbruck christmas market", "subreddit": "Europetravel", "results": 25},
        {"query": "Salzburg christmas market skiing", "subreddit": "travel", "results": 25},
        {"query": "Austria skiing christmas", "subreddit": "travel", "results": 25},
        {"query": "christmas market austria ski", "subreddit": "Europetravel", "results": 25},
        {"query": "Kitzb\u00fchel christmas", "subreddit": "skiing", "results": 2},
        {"query": "advent market alpine town", "subreddit": "travel", "results": 25},
        {"query": "ski trip Austria December", "subreddit": "skiing", "results": 25},
        {"query": "Innsbruck skiing december", "subreddit": "skithealps", "results": 25},
        {"query": "christmas market Garmisch Salzburg Innsbruck", "subreddit": "travel", "results": 25},
        {"query": "Bolzano christmas market south tyrol", "subreddit": "travel", "results": 25},
        {"query": "Garmisch-Partenkirchen christmas", "subreddit": "travel", "results": 3},
        {"query": "christmas market december skiing Austria", "subreddit": "solotravel", "results": 25},
        {"query": "ski holiday alps luxury christmas", "subreddit": "FATTravel", "results": 25},
        {"query": "best christmas market germany austria", "subreddit": "Europetravel", "results": 25},
        {"query": "Zell am See Salzburg skiing christmas", "subreddit": "travel", "results": 25},
        {"query": "St Moritz Davos christmas market", "subreddit": "travel", "results": 25},
        {"query": "alpine village christmas skiing family", "subreddit": "chubbytravel", "results": 25},
        {"query": "December itinerary Austria christmas market skiing", "subreddit": "Europetravel", "results": 25},
        {"query": "cross country skiing christmas alps", "subreddit": "skiing", "results": 25},
        {"query": "Kitzb\u00fchel Innsbruck Salzburg winter trip", "subreddit": "Europetravel", "results": 25},
        {"query": "Dolomites christmas market bolzano merano", "subreddit": "Europetravel", "results": 25},
        {"query": "christmas markets trip report", "subreddit": "Europetravel", "results": 25},
        {"query": "St Anton Lech Z\u00fcrs ski christmas", "subreddit": "skithealps", "results": 25},
        {"query": "austria ski resort family december christmas village", "subreddit": "skithealps", "results": 25},
        {"query": "Garmisch-Partenkirchen skiing december", "subreddit": "germany", "results": 25},
    ]

    output = {"posts": output_posts, "search_log": search_log}

    os.makedirs(os.path.dirname(OUTPUT_PATH), exist_ok=True)
    with open(OUTPUT_PATH, "w") as f:
        json.dump(output, f, indent=2, ensure_ascii=False)

    print(f"\nWrote {len(output_posts)} posts to {OUTPUT_PATH}")

    print("\n\u2500\u2500 Top 20 posts \u2500\u2500")
    for i, p in enumerate(output_posts[:20], 1):
        print(f"  {i:2d}. [{p['subreddit']:15s}] (s:{p['score']:4d} c:{p['num_comments']:3d}) {p['title'][:75]}")
    print(f"\n\u2500\u2500 Posts 21-40 \u2500\u2500")
    for i, p in enumerate(output_posts[20:40], 21):
        print(f"  {i:2d}. [{p['subreddit']:15s}] (s:{p['score']:4d} c:{p['num_comments']:3d}) {p['title'][:75]}")
    print(f"\n\u2500\u2500 Posts 41-60 \u2500\u2500")
    for i, p in enumerate(output_posts[40:60], 41):
        print(f"  {i:2d}. [{p['subreddit']:15s}] (s:{p['score']:4d} c:{p['num_comments']:3d}) {p['title'][:75]}")

    # Subreddit distribution
    from collections import Counter
    subs = Counter(p["subreddit"] for p in output_posts)
    print("\nSubreddit distribution in top 60:")
    for sub, cnt in subs.most_common():
        print(f"  r/{sub}: {cnt}")


if __name__ == "__main__":
    main()
