package com.lingqiao.server.releases;

import java.time.Instant;
import java.util.List;
import java.util.Optional;

/**
 * Picks the release/package pair to offer a client, mirroring Go
 * releases.Selector.Select ordering and filters.
 */
public class ReleaseUpdateService {
    private final ReleaseSelectionStore store;

    public ReleaseUpdateService(ReleaseSelectionStore store) {
        this.store = store;
    }

    public Optional<SelectedRelease> select(ReleaseSelector.ClientContext client) {
        for (Release release : store.listPublished(client.channel())) {
            if (!ReleaseSelector.versionGreater(release.version(), client.version())) {
                continue;
            }
            if (!ReleaseSelector.matchesTargeting(release.targeting(), client)) {
                continue;
            }
            if (!ReleaseSelector.inRollout(release.id(), release.rolloutPercent(), client)) {
                continue;
            }
            List<ReleasePackage> packages = store.listPackages(release.id());
            if (packages.isEmpty()) {
                continue;
            }
            return Optional.of(new SelectedRelease(release, packages.get(0)));
        }
        return Optional.empty();
    }

    public UpdateManifest toManifest(SelectedRelease selected) {
        Release release = selected.release();
        ReleasePackage pkg = selected.pkg();
        return new UpdateManifest(
            release.id(),
            release.version(),
            release.channel(),
            release.minVersion(),
            release.forceUpdate(),
            release.notes(),
            pkg.id(),
            pkg.kind(),
            "/api/v1/update/package/" + pkg.id(),
            pkg.fileSize(),
            pkg.sha256(),
            release.scriptId(),
            release.scriptVersion(),
            release.scriptSha256(),
            release.rolloutPercent(),
            release.createdAt() == null ? Instant.EPOCH : release.createdAt()
        );
    }

    public record SelectedRelease(Release release, ReleasePackage pkg) {
    }
}
