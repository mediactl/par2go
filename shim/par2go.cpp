/*
Copyright 2026 The Clustarr Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/
// System headers come first: par2's libpar2.h includes <string.h> and
// <inttypes.h> inside namespace Par2, which only works when the include
// guards have already seen them (upstream's own files get this from their
// precompiled header, libpar2internal.h).
#include <inttypes.h>
#include <string.h>

#include <atomic>
#include <cstring>
#include <exception>
#include <mutex>
#include <new>
#include <ostream>
#include <streambuf>
#include <string>
#include <utility>
#include <vector>

#include "par2go.h"

#include <par2/libpar2.h>
#include <par2/par2repairer.h>

namespace {

constexpr size_t kLogCap = 4096;

// TailBuf keeps the last kLogCap bytes of par2's output, with each \r
// rewinding to the start of its line as a terminal would, so progress
// meters do not crowd out the messages.
class TailBuf : public std::streambuf {
public:
  std::string str() {
    std::lock_guard<std::mutex> l(mu_);
    return buf_.size() > kLogCap ? buf_.substr(buf_.size() - kLogCap) : buf_;
  }
  void append(const std::string &s) { xsputn(s.data(), static_cast<std::streamsize>(s.size())); }

protected:
  int_type overflow(int_type ch) override {
    if (traits_type::eq_int_type(ch, traits_type::eof()))
      return traits_type::not_eof(ch);
    char c = traits_type::to_char_type(ch);
    xsputn(&c, 1);
    return ch;
  }
  std::streamsize xsputn(const char *s, std::streamsize n) override {
    std::lock_guard<std::mutex> l(mu_);
    for (std::streamsize i = 0; i < n; ++i) {
      if (s[i] == '\r') {
        size_t nl = buf_.rfind('\n');
        buf_.erase(nl == std::string::npos ? 0 : nl + 1);
      } else {
        buf_.push_back(s[i]);
      }
    }
    if (buf_.size() > 2 * kLogCap)
      buf_.erase(0, buf_.size() - kLogCap);
    return n;
  }

private:
  std::mutex mu_;
  std::string buf_;
};

// Streams is a base so it is constructed before Par2Repairer, which keeps
// references to the streams.
struct Streams {
  TailBuf buf;
  std::ostream os{&buf};
};

struct FileRec {
  int64_t state = P2_FILE_MISSING;
  int64_t available = 0;
  int64_t total = 0;
  std::string name;
};

void copyName(const std::string &s, char *dst, size_t cap, int64_t *truncated) {
  size_t n = s.size() < cap - 1 ? s.size() : cap - 1;
  std::memcpy(dst, s.data(), n);
  dst[n] = '\0';
  *truncated = s.size() > n ? 1 : 0;
}

class Job : private Streams, public Par2::Par2Repairer {
public:
  Job(std::string index, std::string base, int64_t mem, int32_t threads, int32_t fthreads, bool purge)
      : Streams(), Par2::Par2Repairer(Streams::os, Streams::os, Par2::nlNormal),
        index_(std::move(index)), base_(std::move(base)), mem_(mem), threads_(threads),
        fthreads_(fthreads), purge_(purge) {
    if (!base_.empty() && base_.back() != '/')
      base_ += '/';
  }

  void addExtra(const char *p) { extras_.emplace_back(p); }

  int32_t run(bool repair) {
    if (mem_ <= 0 || threads_ < 0 || fthreads_ < 0 || index_.empty() || base_.empty())
      return P2_INVALID;
    if (cancel_requested_)
      return P2_CANCELLED;
    // Process ignores its basepath argument (upstream fact 1).
    basepath = base_;
    const size_t mem = static_cast<size_t>(mem_);
    Par2::Result r = Process(mem, base_, static_cast<Par2::u32>(threads_),
                             static_cast<Par2::u32>(fthreads_), index_, extras_,
                             /*dorepair=*/false, /*purgefiles=*/repair && purge_,
                             false, false, 0);
    snapshot();
    if (cancel_requested_)
      return P2_CANCELLED;
    if (repair && r == Par2::eRepairPossible) {
      repair_attempted_ = true;
      // Same object: Process skips loading and verifying (upstream fact 2).
      r = Process(mem, base_, static_cast<Par2::u32>(threads_),
                  static_cast<Par2::u32>(fthreads_), index_, extras_,
                  /*dorepair=*/true, /*purgefiles=*/purge_, false, false, 0);
      if (cancel_requested_)
        return P2_CANCELLED;
    }
    return static_cast<int32_t>(r);
  }

  void cancel() {
    cancel_requested_ = true;
    cancelled = true; // upstream's plain bool, polled by its loops
  }

  void progress(p2_progress *out) {
    out->phase = phase_.load();
    out->per_mille = per_mille_.load();
    std::lock_guard<std::mutex> l(mu_);
    copyName(file_, out->file, sizeof out->file, &out->file_truncated);
  }

  int32_t counts(p2_counts *out) {
    std::lock_guard<std::mutex> l(mu_);
    *out = counts_;
    out->repair_attempted = repair_attempted_ ? 1 : 0;
    out->files = static_cast<int64_t>(files_.size());
    return 0;
  }

  int32_t file(int32_t i, p2_file_result *out) {
    std::lock_guard<std::mutex> l(mu_);
    if (i < 0 || static_cast<size_t>(i) >= files_.size())
      return -1;
    const FileRec &f = files_[static_cast<size_t>(i)];
    out->state = f.state;
    out->blocks_available = f.available;
    out->blocks_total = f.total;
    copyName(f.name, out->name, sizeof out->name, &out->name_truncated);
    return 0;
  }

  std::string log() { return buf.str(); }
  void note(const std::string &s) { buf.append(s); }

protected:
  void SigFilename(std::string filename) override {
    std::lock_guard<std::mutex> l(mu_);
    file_ = std::move(filename);
    per_mille_ = 0;
  }
  void SigProgress(int pm) override { per_mille_ = pm; }
  void BeginRepair() override {
    phase_ = P2_PHASE_REPAIRING;
    per_mille_ = 0;
    std::lock_guard<std::mutex> l(mu_);
    file_.clear();
  }

private:
  // snapshot records what verification found (upstream facts 4 and 5).
  void snapshot() {
    std::lock_guard<std::mutex> l(mu_);
    counts_ = p2_counts{};
    counts_.block_size = static_cast<int64_t>(blocksize);
    counts_.data_blocks = sourceblockcount;
    counts_.recovery_blocks = static_cast<int64_t>(recoverypacketmap.size());
    counts_.complete_files = completefilecount;
    counts_.renamed_files = renamedfilecount;
    counts_.damaged_files = damagedfilecount;
    counts_.missing_files = missingfilecount;
    counts_.available_blocks = availableblockcount;
    counts_.missing_blocks = missingblockcount;
    files_.clear();
    if (mainpacket == nullptr)
      return;
    counts_.recoverable_files = mainpacket->RecoverableFileCount();
    counts_.other_files = mainpacket->TotalFileCount() - mainpacket->RecoverableFileCount();
    std::string id = setid.print();
    std::strncpy(counts_.set_id, id.c_str(), sizeof counts_.set_id - 1);
    if (!alreadyloaded)
      return; // verification never ran: no per-file state to read
    Par2::u32 n = 0;
    for (Par2::Par2RepairerSourceFile *sf : sourcefiles) {
      if (n++ >= mainpacket->TotalFileCount())
        break;
      FileRec r;
      if (sf == nullptr) {
        files_.push_back(r);
        continue;
      }
      r.name = Par2::DiskFile::SplitRelativeFilename(sf->TargetFileName(), basepath);
      r.total = sf->BlockCount();
      if (sf->GetCompleteFile() != nullptr) {
        r.state = sf->GetCompleteFile() == sf->GetTargetFile() ? P2_FILE_COMPLETE : P2_FILE_RENAMED;
        r.available = r.total;
      } else {
        if (blocksallocated) {
          auto sb = sf->SourceBlocks();
          for (Par2::u32 b = 0; b < sf->BlockCount(); ++b, ++sb)
            if (sb->IsSet())
              ++r.available;
        }
        r.state = sf->GetTargetExists() ? P2_FILE_DAMAGED : P2_FILE_MISSING;
      }
      files_.push_back(std::move(r));
    }
  }

  std::string index_, base_;
  int64_t mem_;
  int32_t threads_, fthreads_;
  bool purge_;
  std::vector<std::string> extras_;

  std::atomic<bool> cancel_requested_{false};
  std::atomic<int64_t> phase_{P2_PHASE_VERIFYING};
  std::atomic<int64_t> per_mille_{0};
  bool repair_attempted_ = false;

  std::mutex mu_;
  std::string file_;
  p2_counts counts_{};
  std::vector<FileRec> files_;
};

} // namespace

// Job holds mutexes and atomics, so it is neither copyable nor movable:
// construct it in place.
struct p2_job {
  template <class... A> explicit p2_job(A &&...a) : job(std::forward<A>(a)...) {}
  Job job;
};

extern "C" {

int32_t p2_abi_version(void) { return P2_ABI_VERSION; }

uint64_t p2_sizeof(int32_t which) {
  switch (which) {
  case P2_SIZEOF_PROGRESS: return sizeof(p2_progress);
  case P2_SIZEOF_COUNTS: return sizeof(p2_counts);
  case P2_SIZEOF_FILE: return sizeof(p2_file_result);
  }
  return 0;
}

p2_job *p2_new(const char *index, const char *basepath, int64_t memory_limit, int32_t threads,
               int32_t file_threads, int32_t purge) {
  if (index == nullptr || basepath == nullptr)
    return nullptr;
  try {
    return new p2_job(std::string(index), std::string(basepath), memory_limit, threads,
                      file_threads, purge != 0);
  } catch (...) {
    return nullptr;
  }
}

int32_t p2_add_extra(p2_job *j, const char *path) {
  if (j == nullptr || path == nullptr)
    return -1;
  try {
    j->job.addExtra(path);
    return 0;
  } catch (...) {
    return -1;
  }
}

int32_t p2_run(p2_job *j, int32_t repair) {
  if (j == nullptr)
    return P2_INVALID;
  try {
    return j->job.run(repair != 0);
  } catch (const std::bad_alloc &) {
    j->job.note("\npar2go: out of memory\n");
    return P2_MEMORY_ERROR;
  } catch (const std::exception &e) {
    j->job.note(std::string("\npar2go: exception: ") + e.what() + "\n");
    return P2_LOGIC_ERROR;
  } catch (...) {
    j->job.note("\npar2go: unknown exception\n");
    return P2_LOGIC_ERROR;
  }
}

void p2_progress_read(p2_job *j, p2_progress *out) {
  if (j != nullptr && out != nullptr)
    j->job.progress(out);
}

void p2_cancel(p2_job *j) {
  if (j != nullptr)
    j->job.cancel();
}

int32_t p2_counts_read(p2_job *j, p2_counts *out) {
  return j != nullptr && out != nullptr ? j->job.counts(out) : -1;
}

int32_t p2_file_read(p2_job *j, int32_t i, p2_file_result *out) {
  return j != nullptr && out != nullptr ? j->job.file(i, out) : -1;
}

uint64_t p2_log_read(p2_job *j, char *buf, uint64_t n) {
  if (j == nullptr)
    return 0;
  std::string s = j->job.log();
  if (buf != nullptr && n > 0) {
    size_t c = s.size() < n ? s.size() : static_cast<size_t>(n);
    std::memcpy(buf, s.data(), c);
  }
  return s.size();
}

void p2_free(p2_job *j) { delete j; }

} // extern "C"
