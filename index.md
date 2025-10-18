## Most Active GitHub Users

Discover the most active GitHub contributors from around the world. Our rankings are updated weekly and showcase the top developers, organizations, and contributors across different countries and regions.


### Weekly Rankings
Updated every week with the latest GitHub activity data

### Global Coverage
Rankings available for countries and regions worldwide

### Multiple Metrics
View by commits, contributions, or all activity

### Data Export
Download machine-readable JSON data for analysis

## Browse by Country/Region

<ul class="country-list">
{% assign locations = site.data.locations | sort %}
{% for loc_hash in locations %}
  {% assign location = loc_hash[1] %}
  <li><a href="{{location.page | remove: '.html'}}">{{location.title}}</a></li>
{% endfor %}
</ul>

## Data Export

You can get a combined machine-readable JSON for:
<ul>
<li><a href="rank_only.json"> rank-only with categories</a></li>
</ul>
A subset specific to each country/region is available on the individual page linked above.

### Badges

Badges are also available, which you can include on your profile pages. Simply include the following markdown for users:
```markdown
[![committers.top badge](https://user-badge.committers.top/REGION/USERNAME.svg)](https://user-badge.committers.top/REGION/USERNAME)
```
For organizations, you need to use a slightly different markup:
```markdown
[![committers.top badge](https://org-badge.committers.top/REGION/ORGNAME.svg)](https://org-badge.committers.top/REGION/ORGNAME)
```
In case you aren't currently ranked for a given region, you'll simply receive an "unranked" badge.
