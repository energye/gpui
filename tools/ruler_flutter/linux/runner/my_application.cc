#include "my_application.h"

#include <flutter_linux/flutter_linux.h>
#ifdef GDK_WINDOWING_X11
#include <gdk/gdkx.h>
#endif

#include "flutter/generated_plugin_registrant.h"

struct _MyApplication {
  GtkApplication parent_instance;
  char** dart_entrypoint_arguments;
};

G_DEFINE_TYPE(MyApplication, my_application, GTK_TYPE_APPLICATION)

// P4 ruler probes (matrix rows 7/9/10), runner-side instead of xdotool:
//   RULER_ACTIONS="a@t:spec;..." schedules GTK actions at t seconds from
//   launch, spec one of:
//     minimize   — gtk_window_iconify (row 9: R17 FG→BG at t=22)
//     res:WxH    — gtk_window_resize (row 7: R11 size waves, e.g. two waves
//                  scheduled inside the measured window)
//   RULER_INSTANCE_INDEX=N renames the window "RULER_<SCENE>_<N>" and places
//   it at the same tiling offsets gpui multiwin uses (col=i%4, row=i/4,
//   X=col*480, Y=row*380). Titles stay unique so the measure harness can
//   verify all N windows.
typedef struct {
  GtkWindow* window;
  guint sec;
  gchar spec[32];
} RulerAction;

static gboolean ruler_action_fire(gpointer data) {
  RulerAction* a = (RulerAction*)data;
  if (g_str_has_prefix(a->spec, "res:")) {
    int w = 0, h = 0;
    if (sscanf(a->spec + 4, "%dx%d", &w, &h) == 2 && w > 0 && h > 0) {
      gtk_window_resize(a->window, w, h);
    }
  } else if (g_strcmp0(a->spec, "minimize") == 0) {
    gtk_window_iconify(a->window);
  }
  return G_SOURCE_REMOVE; // one-shot
}

static void ruler_schedule_actions(GtkWindow* window) {
  const gchar* env = g_getenv("RULER_ACTIONS");
  if (env == NULL || env[0] == '\0') return;
  gchar* copy = g_strdup(env);
  // Grammar: "<spec>@<t>[,<spec>@<t>...]" where spec is one of
  //   minimize     — gtk_window_iconify at t seconds from launch (row 9)
  //   res:WxH      — gtk_window_resize to WxH at t seconds (row 7 waves)
  // ',' separators keep Makefiles quoting-free.
  for (gchar* p = copy; *p != '\0'; p++) {
    if (*p == ',') *p = ';';
  }
  gchar** items = g_strsplit(copy, ";", -1);
  for (int i = 0; items[i] != NULL; i++) {
    gchar* item = g_strstrip(items[i]);
    if (item[0] == '\0') continue;
    gchar* at = strchr(item, '@');
    if (at == NULL) continue;
    *at = '\0';
    guint sec = (guint)g_ascii_strtoull(at + 1, NULL, 10);
    gchar* spec = g_strstrip(item);
    if (spec[0] == '\0') continue;
    RulerAction* a = g_new0(RulerAction, 1);
    a->window = window;
    a->sec = sec;
    g_strlcpy(a->spec, spec, sizeof(a->spec));
    g_timeout_add_seconds(a->sec, ruler_action_fire, a);
  }
  g_strfreev(items);
  g_free(copy);
}

// Called when first Flutter frame received.
static void first_frame_cb(MyApplication* self, FlView* view) {
  gtk_widget_show(gtk_widget_get_toplevel(GTK_WIDGET(view)));
}

// Implements GApplication::activate.
static void my_application_activate(GApplication* application) {
  MyApplication* self = MY_APPLICATION(application);
  GtkWindow* window =
      GTK_WINDOW(gtk_application_window_new(GTK_APPLICATION(application)));

  // The ruler compares client areas, so use a plain server-side title bar
  // instead of a GTK header bar. A header bar would consume part of the
  // requested window height and make the Flutter client area smaller than
  // the gpui comparison window.
  const gchar* ruler_scene = g_getenv("RULER_SCENE");
  gchar ruler_title[64] = "Flutter ruler";
  int ruler_width = 1200;
  int ruler_height = 800;
  if (ruler_scene == NULL || ruler_scene[0] == '\0') ruler_scene = "spin";
  if (g_strcmp0(ruler_scene, "spin") == 0) {
    g_strlcpy(ruler_title, "RULER_SPIN", sizeof(ruler_title));
    ruler_width = 480;
    ruler_height = 320;
  } else {
    const gchar* known = "list:text:full:grid:boundary:overlay:resize:min:cp:";
    gchar* prefix = g_strdup_printf("%s:", ruler_scene);
    gboolean is_known = strstr(known, prefix) != NULL;
    g_free(prefix);
    if (is_known) {
      gchar upper[32];
      for (int i = 0; i < 31 && ruler_scene[i] != '\0'; i++) {
        upper[i] = g_ascii_toupper(ruler_scene[i]);
      }
      upper[31] = '\0';
      g_snprintf(ruler_title, sizeof(ruler_title), "RULER_%s", upper);
    } else {
      // Unknown scene keeps a default-size window; Dart falls back to spin.
      ruler_width = 480;
      ruler_height = 320;
    }
  }
  gtk_window_set_title(window, ruler_title);
  gtk_window_set_default_size(window, ruler_width, ruler_height);

  // Row 10 multi-instance arm: unique titles + gpui multiwin tiling offsets
  // (col=i%4, row=i/4, X=col*480, Y=row*380) so N spin windows tile the same
  // way on the measurement desktop.
  const gchar* idx_env = g_getenv("RULER_INSTANCE_INDEX");
  if (idx_env != NULL && idx_env[0] != '\0') {
    int idx = (int)g_ascii_strtoll(idx_env, NULL, 10);
    if (idx > 0) {
      gchar titled[96];
      g_snprintf(titled, sizeof(titled), "%s_%d", ruler_title, idx);
      gtk_window_set_title(window, titled);
      const int gx = ((idx - 1) % 4) * 480;
      const int gy = ((idx - 1) / 4) * (320 + 60);
      gtk_window_move(window, gx, gy);
    }
  }

  // Rows 7/9/10 runner-side probes: GTK resize / iconify at scheduled times.
  ruler_schedule_actions(window);

  g_autoptr(FlDartProject) project = fl_dart_project_new();
  fl_dart_project_set_dart_entrypoint_arguments(
      project, self->dart_entrypoint_arguments);

  FlView* view = fl_view_new(project);
  GdkRGBA background_color;
  // Background defaults to black, override it here if necessary, e.g. #00000000
  // for transparent.
  gdk_rgba_parse(&background_color, "#000000");
  fl_view_set_background_color(view, &background_color);
  gtk_widget_show(GTK_WIDGET(view));
  gtk_container_add(GTK_CONTAINER(window), GTK_WIDGET(view));

  // Show the window when Flutter renders.
  // Requires the view to be realized so we can start rendering.
  g_signal_connect_swapped(view, "first-frame", G_CALLBACK(first_frame_cb),
                           self);
  gtk_widget_realize(GTK_WIDGET(view));

  fl_register_plugins(FL_PLUGIN_REGISTRY(view));

  gtk_widget_grab_focus(GTK_WIDGET(view));
}

// Implements GApplication::local_command_line.
static gboolean my_application_local_command_line(GApplication* application,
                                                  gchar*** arguments,
                                                  int* exit_status) {
  MyApplication* self = MY_APPLICATION(application);
  // Strip out the first argument as it is the binary name.
  self->dart_entrypoint_arguments = g_strdupv(*arguments + 1);

  g_autoptr(GError) error = nullptr;
  if (!g_application_register(application, nullptr, &error)) {
    g_warning("Failed to register: %s", error->message);
    *exit_status = 1;
    return TRUE;
  }

  g_application_activate(application);
  *exit_status = 0;

  return TRUE;
}

// Implements GApplication::startup.
static void my_application_startup(GApplication* application) {
  // MyApplication* self = MY_APPLICATION(object);

  // Perform any actions required at application startup.

  G_APPLICATION_CLASS(my_application_parent_class)->startup(application);
}

// Implements GApplication::shutdown.
static void my_application_shutdown(GApplication* application) {
  // MyApplication* self = MY_APPLICATION(object);

  // Perform any actions required at application shutdown.

  G_APPLICATION_CLASS(my_application_parent_class)->shutdown(application);
}

// Implements GObject::dispose.
static void my_application_dispose(GObject* object) {
  MyApplication* self = MY_APPLICATION(object);
  g_clear_pointer(&self->dart_entrypoint_arguments, g_strfreev);
  G_OBJECT_CLASS(my_application_parent_class)->dispose(object);
}

static void my_application_class_init(MyApplicationClass* klass) {
  G_APPLICATION_CLASS(klass)->activate = my_application_activate;
  G_APPLICATION_CLASS(klass)->local_command_line =
      my_application_local_command_line;
  G_APPLICATION_CLASS(klass)->startup = my_application_startup;
  G_APPLICATION_CLASS(klass)->shutdown = my_application_shutdown;
  G_OBJECT_CLASS(klass)->dispose = my_application_dispose;
}

static void my_application_init(MyApplication* self) {}

MyApplication* my_application_new() {
  // Set the program name to the application ID, which helps various systems
  // like GTK and desktop environments map this running application to its
  // corresponding .desktop file. This ensures better integration by allowing
  // the application to be recognized beyond its binary name.
  g_set_prgname(APPLICATION_ID);

  return MY_APPLICATION(g_object_new(my_application_get_type(),
                                     "application-id", APPLICATION_ID, "flags",
                                     G_APPLICATION_NON_UNIQUE, nullptr));
}
