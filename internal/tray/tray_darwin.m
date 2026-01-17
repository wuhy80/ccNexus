#import <Cocoa/Cocoa.h>

@interface TrayDelegate : NSObject
@property (strong, nonatomic) NSStatusItem *statusItem;
@property (strong, nonatomic) NSMenu *menu;
@property (strong, nonatomic) NSMenuItem *showItem;
@property (strong, nonatomic) NSMenuItem *quitItem;
@property (copy, nonatomic) NSString *currentLang;
@end

@implementation TrayDelegate

- (void)setupTray:(NSData *)iconData language:(NSString *)lang {
    self.currentLang = lang;

    dispatch_async(dispatch_get_main_queue(), ^{
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];

        self.statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength];

        NSImage *icon = [[NSImage alloc] initWithData:iconData];
        [icon setSize:NSMakeSize(18, 18)];
        [self.statusItem.button setImage:icon];
        [self.statusItem.button setImageScaling:NSImageScaleProportionallyDown];

        self.menu = [[NSMenu alloc] init];

        // 显示窗口菜单项
        self.showItem = [[NSMenuItem alloc] initWithTitle:[self showTitle]
                                                   action:@selector(showWindow:)
                                            keyEquivalent:@""];
        [self.showItem setTarget:self];
        [self.menu addItem:self.showItem];

        [self.menu addItem:[NSMenuItem separatorItem]];

        // 退出菜单项
        self.quitItem = [[NSMenuItem alloc] initWithTitle:[self quitTitle]
                                                   action:@selector(quitApp:)
                                            keyEquivalent:@""];
        [self.quitItem setTarget:self];
        [self.menu addItem:self.quitItem];

        [self.statusItem.button setTarget:self];
        [self.statusItem.button setAction:@selector(iconClicked:)];
        [[self.statusItem button] sendActionOn:NSEventMaskLeftMouseUp | NSEventMaskRightMouseUp];
    });
}

- (NSString *)showTitle {
    if ([self.currentLang isEqualToString:@"zh-CN"]) {
        return @"显示窗口";
    }
    return @"Show Window";
}

- (NSString *)quitTitle {
    if ([self.currentLang isEqualToString:@"zh-CN"]) {
        return @"退出程序";
    }
    return @"Quit";
}

- (void)updateLanguage:(NSString *)lang {
    self.currentLang = lang;
    dispatch_async(dispatch_get_main_queue(), ^{
        if (self.showItem != nil) {
            [self.showItem setTitle:[self showTitle]];
        }
        if (self.quitItem != nil) {
            [self.quitItem setTitle:[self quitTitle]];
        }
    });
}

extern void goShowWindow();
extern void goHideWindow();
extern void goQuitApp();

- (void)iconClicked:(id)sender {
    NSEvent *event = [NSApp currentEvent];
    if (event.type == NSEventTypeRightMouseUp) {
        [self.statusItem popUpStatusItemMenu:self.menu];
    } else {
        goShowWindow();
    }
}

- (void)showWindow:(id)sender {
    goShowWindow();
}

- (void)quitApp:(id)sender {
    goQuitApp();
}

@end

static TrayDelegate *trayDelegate = nil;

void setupTray(void *iconData, int iconLen, const char *lang) {
    if (trayDelegate == nil) {
        trayDelegate = [[TrayDelegate alloc] init];
    }
    NSData *data = [NSData dataWithBytes:iconData length:iconLen];
    NSString *langStr = [NSString stringWithUTF8String:lang];
    [trayDelegate setupTray:data language:langStr];
}

void updateTrayLanguage(const char *lang) {
    if (trayDelegate != nil) {
        NSString *langStr = [NSString stringWithUTF8String:lang];
        [trayDelegate updateLanguage:langStr];
    }
}
