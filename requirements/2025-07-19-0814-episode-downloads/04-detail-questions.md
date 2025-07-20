# Detail Questions

## Q6: Should download failures retry automatically with exponential backoff?
**Default if unknown:** Yes (network issues are common; automatic retry with backoff provides better user experience and reduces manual intervention)

## Q7: Will the download feature extend the existing models/subscription.go persistence or require a separate download registry file?
**Default if unknown:** No (separate registry file maintains cleaner separation of concerns and better performance for download operations without affecting subscription loading)

## Q8: Should the download manager support concurrent downloads or download episodes one at a time?
**Default if unknown:** Yes (concurrent downloads make better use of bandwidth and reduce total download time, following common patterns in download managers)

## Q9: Will users need to see detailed download progress (speed, ETA, bytes transferred) or just basic progress indicators?
**Default if unknown:** No (basic progress indicators keep the TUI clean and focused; detailed metrics can overwhelm the terminal interface)

## Q10: Should downloaded episodes be automatically removed from the download queue when the source episode is no longer available in the RSS feed?
**Default if unknown:** No (downloaded episodes should persist even if removed from feeds, as users may want to keep archived content they've already downloaded)